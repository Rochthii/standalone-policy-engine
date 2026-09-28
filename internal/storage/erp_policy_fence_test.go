package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"os"
	"testing"
	"time"

	"standalone-policy-engine/internal/testutil"
)

func TestERPPolicyPublicationFence(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL evidence")
	}
	ctx := context.Background()
	store, err := NewStorage(testutil.CreateIsolatedPostgresDatabase(t, ctx, adminURL))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fence, err := NewERPRevocationStore(store, testutil.CreateIsolatedPostgresDatabase(t, ctx, adminURL))
	if err != nil {
		t.Fatal(err)
	}
	defer fence.Close()
	if _, err := fence.pool.Exec(ctx, `CREATE TABLE pdp_policy_fence_v1
		(tenant_id text PRIMARY KEY, revision bigint NOT NULL, ready boolean NOT NULL, publication_id text)`); err != nil {
		t.Fatal(err)
	}
	store.SetERPPolicyFence(fence)
	assertFence := func(tenant string, ready bool, revision uint64) {
		t.Helper()
		var gotReady bool
		var gotRevision uint64
		if err := fence.pool.QueryRow(ctx, `SELECT ready, revision FROM pdp_policy_fence_v1 WHERE tenant_id=$1`, tenant).Scan(&gotReady, &gotRevision); err != nil {
			t.Fatal(err)
		}
		if gotReady != ready || gotRevision != revision {
			t.Fatalf("fence=(%v,%d), want=(%v,%d)", gotReady, gotRevision, ready, revision)
		}
	}
	for _, kind := range []string{"publish", "update", "delete", "roles"} {
		t.Run(kind, func(t *testing.T) {
			fixture := preparePolicyMutation(t, store, ctx, "fence-"+kind, kind)
			if err := fence.EnsureERPPolicyRevision(ctx, fixture.tenantID, fixture.revision); err != nil {
				t.Fatal(err)
			}
			mutate := func(ctx context.Context) error {
				if kind == "roles" {
					return store.ReplaceRoleInheritances(ctx, fixture.tenantID, [][2]string{{"manager", "buyer"}})
				}
				return applyPolicyMutation(ctx, store, fixture)
			}
			// Real final lock: timeout must not mutate PDP or leak pending state.
			tx, err := fence.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, `SELECT 1 FROM pdp_policy_fence_v1 WHERE tenant_id=$1 FOR UPDATE`, fixture.tenantID); err != nil {
				t.Fatal(err)
			}
			blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			err = mutate(blocked)
			cancel()
			if err == nil {
				t.Fatal("writer bypassed final lock")
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			assertFence(fixture.tenantID, true, fixture.revision)
			assertPolicyMutationRolledBack(t, store, ctx, fixture)
			// The live writer observes the durable unavailable barrier in its
			// own mutation, then releases exactly the newly committed revision.
			store.policyNotifier = func(ctx context.Context, tx pgx.Tx, payload string) error {
				assertFence(fixture.tenantID, false, fixture.revision)
				return postgresPolicyNotifier(ctx, tx, payload)
			}
			if err := mutate(ctx); err != nil {
				t.Fatal(err)
			}
			store.policyNotifier = postgresPolicyNotifier
			revision, err := store.GetTenantRevision(ctx, fixture.tenantID)
			if err != nil {
				t.Fatal(err)
			}
			assertFence(fixture.tenantID, true, revision)
		})
	}
	t.Run("failed publication remains unavailable", func(t *testing.T) {
		fixture := preparePolicyMutation(t, store, ctx, "failed-fence", "publish")
		if err := fence.EnsureERPPolicyRevision(ctx, fixture.tenantID, fixture.revision); err != nil {
			t.Fatal(err)
		}
		err := store.fencedPolicyMutation(ctx, fixture.tenantID, func() error { return errors.New("injected crash boundary") })
		if err == nil {
			t.Fatal("missing failure")
		}
		assertFence(fixture.tenantID, false, fixture.revision)
		if err := fence.EnsureERPPolicyRevision(ctx, fixture.tenantID, 999); err != nil {
			t.Fatal(err)
		}
		assertFence(fixture.tenantID, false, fixture.revision)
		called := false
		if err := store.fencedPolicyMutation(ctx, fixture.tenantID, func() error { called = true; return nil }); err == nil || called {
			t.Fatal("pending publication was bypassed")
		}
	})
}
