package storage

import (
	"context"
	"fmt"
	"github.com/google/uuid"
)

// SetERPPolicyFence configures the same ERP authority database on every policy
// writer. The caller owns the fence lifetime. Configure before serving requests.
func (s *Storage) SetERPPolicyFence(fence *ERPRevocationStore) { s.erpPolicyFence = fence }

// EnsureERPPolicyRevision only bootstraps an absent row; a decision must never
// clear a pending publication or replace a revision installed by a writer.
func (s *ERPRevocationStore) EnsureERPPolicyRevision(ctx context.Context, tenant string, revision uint64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO pdp_policy_fence_v1(tenant_id,revision,ready)
 VALUES($1,$2,true) ON CONFLICT(tenant_id) DO NOTHING`, tenant, revision)
	return err
}

func (s *Storage) fencedPolicyMutation(ctx context.Context, tenant string, mutate func() error) error {
	if s.erpPolicyFence == nil {
		return mutate()
	}
	token := uuid.NewString()
	// Persist the unavailable state BEFORE touching PDP policy storage. A process,
	// transport, or ambiguous commit failure leaves a durable fail-closed barrier.
	tx, err := s.erpPolicyFence.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO pdp_policy_fence_v1(tenant_id,revision,ready)
 VALUES($1,0,true) ON CONFLICT(tenant_id) DO NOTHING`, tenant)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE pdp_policy_fence_v1 SET ready=false, publication_id=$2
 WHERE tenant_id=$1 AND ready=true`, tenant, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("ERP policy publication already pending for tenant %s", tenant)
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = mutate(); err != nil {
		return fmt.Errorf("policy publication remains fenced pending recovery: %w", err)
	}
	revision, err := s.GetTenantRevision(ctx, tenant)
	if err != nil {
		return err
	}
	tag, err = s.erpPolicyFence.pool.Exec(ctx, `UPDATE pdp_policy_fence_v1
 SET revision=$3,ready=true,publication_id=NULL WHERE tenant_id=$1 AND publication_id=$2 AND ready=false`, tenant, token, revision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("ERP policy publication ownership lost")
	}
	return nil
}

func (s *Storage) UpdatePolicy(ctx context.Context, tenantID, policyID, policyText string) error {
	return s.fencedPolicyMutation(ctx, tenantID, func() error { return s.updatePolicy(ctx, tenantID, policyID, policyText) })
}
func (s *Storage) PublishPolicy(ctx context.Context, tenantID, policyID string, astJSON []byte) (int, error) {
	var version int
	err := s.fencedPolicyMutation(ctx, tenantID, func() error {
		var err error
		version, err = s.publishPolicy(ctx, tenantID, policyID, astJSON)
		return err
	})
	return version, err
}
func (s *Storage) DeletePolicy(ctx context.Context, tenantID, policyID string) error {
	return s.fencedPolicyMutation(ctx, tenantID, func() error { return s.deletePolicy(ctx, tenantID, policyID) })
}
func (s *Storage) ReplaceRoleInheritances(ctx context.Context, tenantID string, inheritances [][2]string) error {
	if err := validateRoleInheritances(inheritances); err != nil {
		return err
	}
	return s.fencedPolicyMutation(ctx, tenantID, func() error { return s.replaceRoleInheritances(ctx, tenantID, inheritances) })
}
