package storage

import (
	"context"
	"errors"
	"fmt"

	"standalone-policy-engine/internal/security"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ERPRevocationStore writes a monotonic tombstone in the ERP database BEFORE
// publishing the existing PDP revocation. ERP final transactions lock the same
// row until business commit. No transaction spans the two databases: if PDP
// publication fails, the committed ERP tombstone still denies final execution.
type ERPRevocationStore struct {
	security.RevocationStore
	pool  *pgxpool.Pool
	scope string
}

func NewERPRevocationStore(base security.RevocationStore, connectionString string) (*ERPRevocationStore, error) {
	if base == nil {
		return nil, errors.New("ERP revocation fencing requires durable PDP storage")
	}
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil || cfg.ConnConfig.Database == "" {
		return nil, errors.New("invalid ERP revocation database configuration")
	}
	cfg.MinConns = 0
	cfg.MaxConns = 4
	cfg.ConnConfig.RuntimeParams["application_name"] = "pdp-erp-revocation-fence"
	// Lazy connections allow the Odoo addon to create its own schema. Any missing
	// database/table or connection failure rejects RevokeDelegation at runtime.
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, errors.New("cannot initialize ERP revocation pool")
	}
	return &ERPRevocationStore{
		RevocationStore: base, pool: pool,
		scope: "odoo-revocation.v1:" + cfg.ConnConfig.Database,
	}, nil
}

func (s *ERPRevocationStore) ERPRevocationFenceScope() string { return s.scope }

func (s *ERPRevocationStore) Close() { s.pool.Close() }

func (s *ERPRevocationStore) PersistRevocation(ctx context.Context, record security.RevocationRecord) (security.RevocationRecord, error) {
	if record.TenantID == "" || record.GrantID == "" {
		return security.RevocationRecord{}, errors.New("ERP revocation requires tenant and grant")
	}
	// Use an explicit transaction: canceling a blocked auto-commit statement
	// alone could still let PostgreSQL execute it as the blocking lock releases.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return security.RevocationRecord{}, fmt.Errorf("begin ERP revocation fence: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO pdp_delegation_fence_v1 (tenant_id, grant_id, revoked)
		VALUES ($1, $2, true)
		ON CONFLICT (tenant_id, grant_id) DO UPDATE SET revoked = true`, record.TenantID, record.GrantID)
	if err != nil {
		return security.RevocationRecord{}, fmt.Errorf("commit ERP revocation fence: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return security.RevocationRecord{}, fmt.Errorf("commit ERP revocation fence: %w", err)
	}
	return s.RevocationStore.PersistRevocation(ctx, record)
}
