package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"standalone-policy-engine/internal/audit"

	"github.com/jackc/pgx/v5"
)

func TestStorage_MigrationsIntegration(t *testing.T) {
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		t.Skip("Bo qua test tich hop Postgres vi bien TEST_DATABASE_URL chua duoc thiet lap")
		return
	}

	ctx := context.Background()
	testDBConnStr := createIsolatedTestDatabase(t, ctx, connStr)

	// Khoi tao Storage - qua trinh nay se tu dong chay runMigrations
	store, err := NewStorage(testDBConnStr)
	if err != nil {
		t.Fatalf("Khoi tao Storage va chay migration that bai: %v", err)
	}
	defer store.Close()

	// Kiem tra cac bang da duoc tao thanh cong chua
	var exists bool
	err = store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'policies'
		);
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Kiem tra bang 'policies' gap loi: %v", err)
	}
	if !exists {
		t.Error("Bang 'policies' khong duoc tao boi migrations")
	}

	err = store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'decision_audit_logs'
		);
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Kiem tra bang 'decision_audit_logs' gap loi: %v", err)
	}
	if !exists {
		t.Error("Bang 'decision_audit_logs' khong duoc tao boi migrations")
	}
	err = store.pool.QueryRow(ctx, `SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'role_inheritances');`).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("role_inheritances migration missing: exists=%v err=%v", exists, err)
	}

	tenantA, err := store.CreateTenant(ctx, "tenant-isolation-a")
	if err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	tenantB, err := store.CreateTenant(ctx, "tenant-isolation-b")
	if err != nil {
		t.Fatalf("create tenant B: %v", err)
	}
	policyID, err := store.CreatePolicy(ctx, tenantA, "PERMIT", "original policy")
	if err != nil {
		t.Fatalf("create tenant A policy: %v", err)
	}

	if _, err := store.GetPolicy(ctx, tenantB, policyID); !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("cross-tenant GetPolicy must return ErrPolicyNotFound, got %v", err)
	}
	if err := store.UpdatePolicy(ctx, tenantB, policyID, "forged update"); !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("cross-tenant UpdatePolicy must return ErrPolicyNotFound, got %v", err)
	}
	if _, err := store.PublishPolicy(ctx, tenantB, policyID, []byte(`{}`)); !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("cross-tenant PublishPolicy must return ErrPolicyNotFound, got %v", err)
	}
	if err := store.DeletePolicy(ctx, tenantB, policyID); !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("cross-tenant DeletePolicy must return ErrPolicyNotFound, got %v", err)
	}

	policy, err := store.GetPolicy(ctx, tenantA, policyID)
	if err != nil {
		t.Fatalf("tenant A must still read its policy: %v", err)
	}
	if policy.PolicyText != "original policy" || policy.Status != "DRAFT" {
		t.Fatalf("cross-tenant operations changed policy: text=%q status=%q", policy.PolicyText, policy.Status)
	}

	revisionBefore, err := store.GetTenantRevision(ctx, tenantA)
	if err != nil {
		t.Fatalf("read initial tenant revision: %v", err)
	}
	if _, err := store.PublishPolicy(ctx, tenantA, policyID, []byte(`{"compiled":true}`)); err != nil {
		t.Fatalf("publish tenant A policy: %v", err)
	}
	revisionAfterPublish, err := store.GetTenantRevision(ctx, tenantA)
	if err != nil || revisionAfterPublish != revisionBefore+1 {
		t.Fatalf("publish revision: before=%d after=%d err=%v", revisionBefore, revisionAfterPublish, err)
	}
	if err := store.UpdatePolicy(ctx, tenantA, policyID, "updated draft policy"); err != nil {
		t.Fatalf("update active policy to draft: %v", err)
	}
	revisionAfterDraft, err := store.GetTenantRevision(ctx, tenantA)
	if err != nil || revisionAfterDraft != revisionAfterPublish+1 {
		t.Fatalf("ACTIVE->DRAFT revision: publish=%d draft=%d err=%v", revisionAfterPublish, revisionAfterDraft, err)
	}
	policy, err = store.GetPolicy(ctx, tenantA, policyID)
	if err != nil || policy.Status != "DRAFT" || len(policy.ASTJSON) != 0 {
		t.Fatalf("updated policy must be clean DRAFT: policy=%#v err=%v", policy, err)
	}

	roles := [][2]string{{"user:alice", "role:manager"}, {"role:manager", "role:staff"}}
	revisionBeforeRoles := revisionAfterDraft
	if err := store.ReplaceRoleInheritances(ctx, tenantA, roles); err != nil {
		t.Fatalf("replace role inheritances: %v", err)
	}
	bundle, err := store.GetTenantPolicyBundle(ctx, tenantA)
	if err != nil {
		t.Fatalf("load tenant policy bundle: %v", err)
	}
	if bundle.Revision != revisionBeforeRoles+1 || len(bundle.Inheritances) != len(roles) {
		t.Fatalf("role bundle mismatch: revision=%d inheritances=%v", bundle.Revision, bundle.Inheritances)
	}
	tenantBBundle, err := store.GetTenantPolicyBundle(ctx, tenantB)
	if err != nil || len(tenantBBundle.Inheritances) != 0 {
		t.Fatalf("role inheritance leaked across tenant: bundle=%#v err=%v", tenantBBundle, err)
	}
	if err := store.ReplaceRoleInheritances(ctx, tenantA, [][2]string{{"role:a", "role:b"}, {"role:b", "role:a"}}); err == nil {
		t.Fatal("cyclic role graph must be rejected")
	}

	auditLogger, err := audit.NewBatchAuditLogger(store, audit.BatchConfig{
		QueueCapacity: 8,
		BatchSize:     2,
		FlushInterval: time.Hour,
		WriteTimeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("create durable audit logger: %v", err)
	}
	auditLogger.Start(ctx)
	auditLogger.Log(revisionAfterDraft, tenantA, "user:alice", "READ", "invoice:42", "ALLOW", policyID, map[string]string{
		"delegation_proof": "raw-proof-must-not-reach-postgres",
		"department":       "Finance",
	})
	auditLogger.Stop()
	if stats := auditLogger.Stats(); stats.Written != 1 || stats.Dropped != 0 || stats.WriteFailures != 0 {
		t.Fatalf("durable audit stats mismatch: %+v", stats)
	}
	var persistedContext []byte
	if err := store.pool.QueryRow(ctx, `
		SELECT evaluated_context
		FROM decision_audit_logs
		WHERE tenant_id = $1 AND matched_policy_id = $2
	`, tenantA, policyID).Scan(&persistedContext); err != nil {
		t.Fatalf("read durable audit entry: %v", err)
	}
	if strings.Contains(string(persistedContext), "raw-proof-must-not-reach-postgres") ||
		!strings.Contains(string(persistedContext), "[REDACTED]") {
		t.Fatalf("persisted audit context was not redacted: %s", persistedContext)
	}
}

func createIsolatedTestDatabase(t *testing.T, ctx context.Context, adminConnStr string) string {
	t.Helper()

	adminURL, err := url.Parse(adminConnStr)
	if err != nil || (adminURL.Scheme != "postgres" && adminURL.Scheme != "postgresql") {
		t.Fatalf("TEST_DATABASE_URL phai la PostgreSQL URL hop le: %v", err)
	}
	adminConn, err := pgx.Connect(ctx, adminConnStr)
	if err != nil {
		t.Fatalf("Khong the ket noi PostgreSQL test admin: %v", err)
	}

	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		adminConn.Close(ctx)
		t.Fatalf("Khong the tao ten database ngau nhien: %v", err)
	}
	testDBName := "policy_engine_test_" + hex.EncodeToString(randomBytes)
	quotedDBName := pgx.Identifier{testDBName}.Sanitize()
	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+quotedDBName); err != nil {
		adminConn.Close(ctx)
		t.Fatalf("Khong the tao database kiem thu rieng %s: %v", testDBName, err)
	}

	t.Cleanup(func() {
		defer adminConn.Close(ctx)
		if !strings.HasPrefix(testDBName, "policy_engine_test_") {
			t.Errorf("Tu choi xoa database ngoai namespace test: %s", testDBName)
			return
		}
		if _, err := adminConn.Exec(ctx, "DROP DATABASE "+quotedDBName+" WITH (FORCE)"); err != nil {
			t.Errorf("Khong the don database kiem thu rieng %s: %v", testDBName, err)
		}
	})

	testURL := *adminURL
	testURL.Path = "/" + testDBName
	return testURL.String()
}
