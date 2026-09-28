package storage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/testutil"
)

type failingPDPRevocationStore struct{ calls int }

func (s *failingPDPRevocationStore) PersistRevocation(context.Context, security.RevocationRecord) (security.RevocationRecord, error) {
	s.calls++
	return security.RevocationRecord{}, errors.New("injected PDP storage failure")
}

func (*failingPDPRevocationStore) WatchRevocations(context.Context, func([]security.RevocationRecord), func(security.RevocationRecord)) error {
	return nil
}

func TestERPRevocationFenceDurabilityAndCancellation(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL fence evidence")
	}
	ctx := context.Background()
	dsn := testutil.CreateIsolatedPostgresDatabase(t, ctx, adminURL)
	base := &failingPDPRevocationStore{}
	store, err := NewERPRevocationStore(base, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	_, err = store.pool.Exec(ctx, `CREATE TABLE pdp_delegation_fence_v1 (
		tenant_id text NOT NULL, grant_id text NOT NULL, revoked boolean NOT NULL,
		PRIMARY KEY (tenant_id, grant_id))`)
	if err != nil {
		t.Fatal(err)
	}
	record := security.RevocationRecord{TenantID: "tenant-a", GrantID: "1"}
	if _, err := store.PersistRevocation(ctx, record); err == nil {
		t.Fatal("PDP storage failure must not report successful revoke")
	}
	var revoked bool
	err = store.pool.QueryRow(ctx, `SELECT revoked FROM pdp_delegation_fence_v1
		WHERE tenant_id=$1 AND grant_id=$2`, record.TenantID, record.GrantID).Scan(&revoked)
	if err != nil || !revoked || base.calls != 1 {
		t.Fatalf("ERP tombstone must survive failed PDP publication: revoked=%v calls=%d err=%v", revoked, base.calls, err)
	}
	// A different tenant using the same grant ID must retain its own fence.
	_, err = store.pool.Exec(ctx, `INSERT INTO pdp_delegation_fence_v1 VALUES ('tenant-b','1',false)`)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT revoked FROM pdp_delegation_fence_v1
		WHERE tenant_id='tenant-b' AND grant_id='1' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	record.TenantID = "tenant-b"
	if _, err := store.PersistRevocation(blocked, record); err == nil || base.calls != 1 {
		t.Fatal("blocked/canceled ERP revoke must not publish a PDP revocation")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	err = store.pool.QueryRow(ctx, `SELECT revoked FROM pdp_delegation_fence_v1
		WHERE tenant_id='tenant-b' AND grant_id='1'`).Scan(&revoked)
	if err != nil || revoked {
		t.Fatalf("canceled revoke leaked a tombstone: revoked=%v err=%v", revoked, err)
	}
}
