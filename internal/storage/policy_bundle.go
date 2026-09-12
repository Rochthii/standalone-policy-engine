package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TenantPolicyBundle is one repeatable-read snapshot used to build immutable
// in-memory state without mixing policy, role and revision versions.
type TenantPolicyBundle struct {
	Policies     []*DBPolicy
	Inheritances [][2]string
	Revision     uint64
}

func (s *Storage) GetTenantPolicyBundle(ctx context.Context, tenantID string) (*TenantPolicyBundle, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	policyRows, err := tx.Query(ctx, `SELECT id, tenant_id, effect, policy_text, ast_json, version, status, created_at, updated_at
                                      FROM policies WHERE tenant_id = $1 AND status = 'ACTIVE';`, tenantID)
	if err != nil {
		return nil, err
	}
	policies := make([]*DBPolicy, 0)
	for policyRows.Next() {
		policy := &DBPolicy{}
		if err := policyRows.Scan(&policy.ID, &policy.TenantID, &policy.Effect, &policy.PolicyText, &policy.ASTJSON, &policy.Version, &policy.Status, &policy.CreatedAt, &policy.UpdatedAt); err != nil {
			policyRows.Close()
			return nil, err
		}
		policies = append(policies, policy)
	}
	if err := policyRows.Err(); err != nil {
		policyRows.Close()
		return nil, err
	}
	policyRows.Close()

	roleRows, err := tx.Query(ctx, `SELECT parent, child FROM role_inheritances WHERE tenant_id = $1 ORDER BY parent, child;`, tenantID)
	if err != nil {
		return nil, err
	}
	inheritances := make([][2]string, 0)
	for roleRows.Next() {
		var pair [2]string
		if err := roleRows.Scan(&pair[0], &pair[1]); err != nil {
			roleRows.Close()
			return nil, err
		}
		inheritances = append(inheritances, pair)
	}
	if err := roleRows.Err(); err != nil {
		roleRows.Close()
		return nil, err
	}
	roleRows.Close()

	var revision uint64
	if err := tx.QueryRow(ctx, `SELECT revision FROM tenants WHERE id = $1;`, tenantID).Scan(&revision); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tenant not found: %s", tenantID)
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &TenantPolicyBundle{Policies: policies, Inheritances: inheritances, Revision: revision}, nil
}

// ReplaceRoleInheritances atomically replaces one tenant's role graph and
// advances the same revision/event stream used by policy changes.
func (s *Storage) ReplaceRoleInheritances(ctx context.Context, tenantID string, inheritances [][2]string) error {
	if err := validateRoleInheritances(inheritances); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM role_inheritances WHERE tenant_id = $1;`, tenantID); err != nil {
		return err
	}
	for _, pair := range inheritances {
		if _, err := tx.Exec(ctx, `INSERT INTO role_inheritances (tenant_id, parent, child) VALUES ($1, $2, $3);`, tenantID, pair[0], pair[1]); err != nil {
			return err
		}
	}
	if _, err := incrementTenantRevisionAndNotify(ctx, tx, tenantID, "role-inheritance", "ROLE_UPDATE"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validateRoleInheritances(inheritances [][2]string) error {
	edges := make(map[string][]string, len(inheritances))
	seen := make(map[[2]string]struct{}, len(inheritances))
	for _, pair := range inheritances {
		parent, child := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
		if parent == "" || child == "" || parent != pair[0] || child != pair[1] {
			return fmt.Errorf("role inheritance endpoints must be non-empty and trimmed")
		}
		if len(parent) > 255 || len(child) > 255 {
			return fmt.Errorf("role inheritance endpoint exceeds 255 characters")
		}
		if parent == child {
			return fmt.Errorf("role inheritance self-edge is not allowed: %s", parent)
		}
		if _, duplicate := seen[pair]; duplicate {
			return fmt.Errorf("duplicate role inheritance: %s -> %s", parent, child)
		}
		seen[pair] = struct{}{}
		edges[parent] = append(edges[parent], child)
	}

	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(string) error
	visit = func(node string) error {
		if visiting[node] {
			return fmt.Errorf("role inheritance cycle detected at %s", node)
		}
		if visited[node] {
			return nil
		}
		visiting[node] = true
		for _, child := range edges[node] {
			if err := visit(child); err != nil {
				return err
			}
		}
		visiting[node] = false
		visited[node] = true
		return nil
	}
	for parent := range edges {
		if err := visit(parent); err != nil {
			return err
		}
	}
	return nil
}
