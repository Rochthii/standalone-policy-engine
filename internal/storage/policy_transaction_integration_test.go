package storage

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

type policyMutationFixture struct {
	kind     string
	tenantID string
	policyID string
	revision uint64
	policy   *DBPolicy
}

func TestStoragePolicyMutationsRollbackOnPostgresFaults(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("PostgreSQL integration requires TEST_DATABASE_URL")
	}
	ctx := context.Background()
	store, err := NewStorage(createIsolatedTestDatabase(t, ctx, adminURL))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	runCases := func(t *testing.T, fixtures []policyMutationFixture) {
		for _, fixture := range fixtures {
			fixture := fixture
			t.Run(fixture.kind, func(t *testing.T) {
				if err := applyPolicyMutation(ctx, store, fixture); err == nil {
					t.Fatal("expected injected PostgreSQL failure")
				}
				assertPolicyMutationRolledBack(t, store, ctx, fixture)
			})
		}
	}

	t.Run("revision update failure", func(t *testing.T) {
		fixtures := preparePolicyMutationFixtures(t, store, ctx, "revision")
		if _, err := store.pool.Exec(ctx, `
			CREATE OR REPLACE FUNCTION fail_policy_revision() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'injected revision failure'; END;
			$$;
			CREATE TRIGGER fail_policy_revision BEFORE UPDATE OF revision ON tenants
			FOR EACH ROW EXECUTE FUNCTION fail_policy_revision();
		`); err != nil {
			t.Fatal(err)
		}
		runCases(t, fixtures)
		if _, err := store.pool.Exec(ctx, `DROP TRIGGER fail_policy_revision ON tenants; DROP FUNCTION fail_policy_revision();`); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("notifier SQL failure", func(t *testing.T) {
		fixtures := preparePolicyMutationFixtures(t, store, ctx, "notifier")
		store.policyNotifier = func(ctx context.Context, tx pgx.Tx, _ string) error {
			_, err := tx.Exec(ctx, `SELECT 1 / 0`)
			return err
		}
		defer func() { store.policyNotifier = postgresPolicyNotifier }()
		runCases(t, fixtures)
	})
}

func preparePolicyMutationFixtures(t *testing.T, store *Storage, ctx context.Context, prefix string) []policyMutationFixture {
	t.Helper()
	fixtures := make([]policyMutationFixture, 0, 3)
	for _, kind := range []string{"publish", "update", "delete"} {
		fixtures = append(fixtures, preparePolicyMutation(t, store, ctx, prefix+"-"+kind, kind))
	}
	return fixtures
}

func preparePolicyMutation(t *testing.T, store *Storage, ctx context.Context, name, kind string) policyMutationFixture {
	t.Helper()
	tenantID, err := store.CreateTenant(ctx, "rollback-"+name)
	if err != nil {
		t.Fatal(err)
	}
	policyID, err := store.CreatePolicy(ctx, tenantID, "PERMIT", "original policy")
	if err != nil {
		t.Fatal(err)
	}
	if kind != "publish" {
		if _, err := store.PublishPolicy(ctx, tenantID, policyID, []byte(`{"compiled":true}`)); err != nil {
			t.Fatal(err)
		}
	}
	revision, err := store.GetTenantRevision(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := store.GetPolicy(ctx, tenantID, policyID)
	if err != nil {
		t.Fatal(err)
	}
	return policyMutationFixture{kind: kind, tenantID: tenantID, policyID: policyID, revision: revision, policy: policy}
}

func applyPolicyMutation(ctx context.Context, store *Storage, fixture policyMutationFixture) error {
	switch fixture.kind {
	case "publish":
		_, err := store.PublishPolicy(ctx, fixture.tenantID, fixture.policyID, []byte(`{"compiled":false}`))
		return err
	case "update":
		return store.UpdatePolicy(ctx, fixture.tenantID, fixture.policyID, "mutated policy")
	case "delete":
		return store.DeletePolicy(ctx, fixture.tenantID, fixture.policyID)
	default:
		return nil
	}
}

func assertPolicyMutationRolledBack(t *testing.T, store *Storage, ctx context.Context, fixture policyMutationFixture) {
	t.Helper()
	policy, err := store.GetPolicy(ctx, fixture.tenantID, fixture.policyID)
	if err != nil {
		t.Fatal(err)
	}
	if policy.PolicyText != fixture.policy.PolicyText || policy.Status != fixture.policy.Status ||
		!bytes.Equal(policy.ASTJSON, fixture.policy.ASTJSON) || policy.Version != fixture.policy.Version {
		t.Fatalf("%s rollback failed: %#v", fixture.kind, policy)
	}
	revision, err := store.GetTenantRevision(ctx, fixture.tenantID)
	if err != nil || revision != fixture.revision {
		t.Fatalf("revision changed after rollback: before=%d after=%d err=%v", fixture.revision, revision, err)
	}
}
