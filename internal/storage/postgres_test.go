package storage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/testutil"
)

func TestStorage_MigrationsIntegration(t *testing.T) {
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		t.Skip("Bo qua test tich hop Postgres vi bien TEST_DATABASE_URL chua duoc thiet lap")
		return
	}

	ctx := context.Background()
	testDBConnStr := testutil.CreateIsolatedPostgresDatabase(t, ctx, connStr)

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

	auditCrypto, err := security.NewEnvelopeCryptoWithKeyring("audit-test", map[string]string{
		"audit-test": "test-audit-kek-new-32-bytes-key!",
	})
	if err != nil {
		t.Fatal(err)
	}
	auditLogger, err := audit.NewBatchAuditLogger(store, audit.BatchConfig{
		QueueCapacity: 8,
		BatchSize:     2,
		FlushInterval: time.Hour,
		WriteTimeout:  time.Second,
		SpillDir:      t.TempDir(),
		SpillMaxBytes: 1 << 20,
		Crypto:        auditCrypto,
	})
	if err != nil {
		t.Fatalf("create durable audit logger: %v", err)
	}
	auditLogger.Start(ctx)
	auditLogger.Log(revisionAfterDraft, tenantA, "user:alice", "READ", "invoice:42", "ALLOW", policyID, map[string]string{
		"delegation_proof": "raw-proof-must-not-reach-postgres",
		"department":       "Finance",
		"request_id":       "req-storage-42",
		"trace_id":         "trace-storage-42",
	})
	auditLogger.Stop()
	if stats := auditLogger.Stats(); stats.Written != 1 || stats.Dropped != 0 || stats.WriteFailures != 0 {
		t.Fatalf("durable audit stats mismatch: %+v", stats)
	}
	var persisted audit.LogEntry
	var subject, action, resource *string
	var persistedContext []byte
	if err := store.pool.QueryRow(ctx, `
		SELECT audit_id::text, event_timestamp_ns, revision_id, payload_version,
			tenant_id::text, decision, COALESCE(matched_policy_id::text, ''), key_id,
			COALESCE(request_id, ''), COALESCE(trace_id, ''), encrypted_dek,
			encrypted_payload, integrity_tag, is_encrypted,
			request_subject, request_action, request_resource, evaluated_context
		FROM decision_audit_logs
		WHERE tenant_id = $1 AND matched_policy_id = $2
	`, tenantA, policyID).Scan(
		&persisted.AuditID, &persisted.Timestamp, &persisted.RevisionID, &persisted.PayloadVersion,
		&persisted.TenantID, &persisted.Decision, &persisted.MatchedPolicyID, &persisted.KeyID,
		&persisted.RequestID, &persisted.TraceID, &persisted.EncryptedDEK,
		&persisted.EncryptedPayload, &persisted.IntegrityTag, &persisted.IsEncrypted,
		&subject, &action, &resource, &persistedContext,
	); err != nil {
		t.Fatalf("read durable audit entry: %v", err)
	}
	if subject != nil || action != nil || resource != nil || persistedContext != nil {
		t.Fatal("encrypted audit record leaked plaintext columns")
	}
	payload, err := audit.VerifyAndDecryptEntry(auditCrypto, &persisted)
	if err != nil {
		t.Fatalf("verify persisted audit envelope: %v", err)
	}
	if payload.Context["delegation_proof"] != "[REDACTED]" || payload.Context["department"] != "Finance" {
		t.Fatalf("decrypted audit context mismatch: %#v", payload.Context)
	}
	if err := store.InsertAuditLogsBatch(ctx, []*audit.LogEntry{&persisted}); err != nil {
		t.Fatalf("idempotent replay insert failed: %v", err)
	}
	var duplicateCount int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM decision_audit_logs WHERE audit_id = $1`, persisted.AuditID).Scan(&duplicateCount); err != nil || duplicateCount != 1 {
		t.Fatalf("audit replay must be idempotent: count=%d err=%v", duplicateCount, err)
	}
}
