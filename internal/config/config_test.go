package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_LoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load config that bai: %v", err)
	}

	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("Kỳ vọng HTTPPort = 8080, thực tế: %d", cfg.Server.HTTPPort)
	}
	if cfg.Server.GRPCPort != 50051 {
		t.Errorf("Kỳ vọng GRPCPort = 50051, thực tế: %d", cfg.Server.GRPCPort)
	}
	if cfg.Server.EvaluationTimeout != 100*time.Millisecond || cfg.Server.GRPCMaxReceiveBytes != 1024*1024 || cfg.Server.GRPCMaxSendBytes != 1024*1024 {
		t.Fatalf("unexpected gRPC runtime defaults: %+v", cfg.Server)
	}
	if cfg.Engine.StorageMode != "cloud" {
		t.Errorf("Kỳ vọng StorageMode = cloud, thực tế: %s", cfg.Engine.StorageMode)
	}
	if cfg.Engine.ReconcileInterval != 10*time.Second {
		t.Errorf("Kỳ vọng ReconcileInterval = 10s, thực tế: %s", cfg.Engine.ReconcileInterval)
	}
	if cfg.Audit.QueueCapacity != 8192 || cfg.Audit.BatchSize != 128 {
		t.Fatalf("unexpected audit queue defaults: %+v", cfg.Audit)
	}
	if cfg.Audit.FlushInterval != 100*time.Millisecond || cfg.Audit.WriteTimeout != 2*time.Second {
		t.Fatalf("unexpected audit timing defaults: %+v", cfg.Audit)
	}
	if cfg.Audit.SpillMaxBytes != 1<<30 || cfg.Security.AuditActiveKeyID != "legacy" {
		t.Fatalf("unexpected audit durability defaults: audit=%+v security=%+v", cfg.Audit, cfg.Security)
	}
	if cfg.Audit.ArchiveBucket != "" || cfg.Audit.ArchivePrefix != "" || cfg.Audit.ArchiveRegion != "" || cfg.Audit.ArchiveExpectedBucketOwner != "" {
		t.Fatalf("archive must be disabled by default: %+v", cfg.Audit)
	}
	if cfg.Security.DelegationActiveKeyID != "legacy" || len(cfg.Security.DelegationKeys) != 1 {
		t.Fatalf("unexpected development delegation key ring: %+v", cfg.Security)
	}
}

func TestConfigRejectsInvalidAuditBatchSettings(t *testing.T) {
	t.Setenv("AUDIT_QUEUE_CAPACITY", "4")
	t.Setenv("AUDIT_BATCH_SIZE", "5")
	if _, err := Load(); err == nil {
		t.Fatal("expected audit batch larger than queue to fail")
	}
}

func TestConfigRejectsInvalidGRPCRuntimeLimits(t *testing.T) {
	t.Setenv("GRPC_MAX_RECEIVE_BYTES", "512")
	if _, err := Load(); err == nil {
		t.Fatal("expected undersized gRPC receive limit to fail")
	}
}

func TestConfigRejectsNonPositiveReconcileInterval(t *testing.T) {
	t.Setenv("SYNC_RECONCILE_INTERVAL", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("expected non-positive reconcile interval to fail")
	}
}

func TestConfigRuntimeEnvironmentBindings(t *testing.T) {
	values := map[string]string{
		"APP_ENV":                             "test",
		"HTTP_PORT":                           "18080",
		"GRPC_PORT":                           "15051",
		"GRPC_EVALUATION_TIMEOUT":             "250ms",
		"GRPC_MAX_RECEIVE_BYTES":              "4096",
		"GRPC_MAX_SEND_BYTES":                 "8192",
		"LISTEN_SOCKET_PATH":                  "/tmp/pdp.sock",
		"USE_ZITI":                            "true",
		"ZITI_IDENTITY_PATH":                  "/run/ziti/pdp.json",
		"ZITI_SERVICE_NAME":                   "pdp-private-service",
		"DATABASE_URL":                        "postgres://test:secret@db.internal:5432/pdp?sslmode=require",
		"STORAGE_MODE":                        "edge",
		"BADGER_DATA_DIR":                     "/var/lib/pdp/badger",
		"DISABLE_GC":                          "true",
		"GC_INTERVAL":                         "2m",
		"GC_IDLE_TIMEOUT":                     "3m",
		"SYNC_RECONCILE_INTERVAL":             "4s",
		"AUDIT_SPILL_DIR":                     "/var/lib/pdp/audit-spill",
		"AUDIT_QUEUE_CAPACITY":                "64",
		"AUDIT_BATCH_SIZE":                    "8",
		"AUDIT_FLUSH_INTERVAL":                "150ms",
		"AUDIT_WRITE_TIMEOUT":                 "3s",
		"AUDIT_SPILL_MAX_BYTES":               "2048",
		"AUDIT_ARCHIVE_BUCKET":                "pdp-audit-archive-test-123",
		"AUDIT_ARCHIVE_PREFIX":                "pdp-audit",
		"AUDIT_ARCHIVE_REGION":                "ap-southeast-1",
		"AUDIT_ARCHIVE_EXPECTED_BUCKET_OWNER": "123456789012",
		"JWT_SECRET":                          "test-jwt-secret-at-least-32-characters",
		"JWT_ISSUER":                          "https://issuer.example.test",
		"JWT_AUDIENCE":                        "pdp-test",
		"PDP_SHARED_SECRET":                   "test-delegation-secret-at-least-32-characters",
		"PDP_DELEGATION_ACTIVE_KID":           "delegation-a",
		"PDP_DELEGATION_KEYS_JSON":            `{"delegation-a":"test-delegation-secret-at-least-32-characters"}`,
		"LOG_KEK_ACTIVE_KID":                  "audit-a",
		"LOG_KEKS_JSON":                       `{"audit-a":"test-audit-kek-new-32-bytes-key!"}`,
		"PDP_TLS_CERT":                        "/run/tls/server.crt",
		"PDP_TLS_KEY":                         "/run/tls/server.key",
		"PDP_TLS_CA":                          "/run/tls/ca.crt",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != "test" || cfg.Database.URL != values["DATABASE_URL"] ||
		cfg.Server.HTTPPort != 18080 || cfg.Server.GRPCPort != 15051 ||
		cfg.Server.EvaluationTimeout != 250*time.Millisecond ||
		cfg.Server.GRPCMaxReceiveBytes != 4096 || cfg.Server.GRPCMaxSendBytes != 8192 ||
		cfg.Server.SocketPath != "/tmp/pdp.sock" || !cfg.Server.UseZiti ||
		cfg.Server.ZitiIdentityPath != "/run/ziti/pdp.json" || cfg.Server.ZitiServiceName != "pdp-private-service" {
		t.Fatalf("server/database environment binding mismatch: %+v", cfg)
	}
	if cfg.Engine.StorageMode != "edge" || cfg.Engine.BadgerDir != "/var/lib/pdp/badger" ||
		!cfg.Engine.DisableGC || cfg.Engine.GCInterval != 2*time.Minute ||
		cfg.Engine.GCIdle != 3*time.Minute || cfg.Engine.ReconcileInterval != 4*time.Second {
		t.Fatalf("engine environment binding mismatch: %+v", cfg.Engine)
	}
	if cfg.Audit.SpillDir != "/var/lib/pdp/audit-spill" || cfg.Audit.QueueCapacity != 64 ||
		cfg.Audit.BatchSize != 8 || cfg.Audit.FlushInterval != 150*time.Millisecond ||
		cfg.Audit.WriteTimeout != 3*time.Second || cfg.Audit.SpillMaxBytes != 2048 ||
		cfg.Audit.ArchiveBucket != values["AUDIT_ARCHIVE_BUCKET"] || cfg.Audit.ArchivePrefix != values["AUDIT_ARCHIVE_PREFIX"] ||
		cfg.Audit.ArchiveRegion != values["AUDIT_ARCHIVE_REGION"] || cfg.Audit.ArchiveExpectedBucketOwner != values["AUDIT_ARCHIVE_EXPECTED_BUCKET_OWNER"] {
		t.Fatalf("audit environment binding mismatch: %+v", cfg.Audit)
	}
	if cfg.Security.JWTSecret != values["JWT_SECRET"] || cfg.Security.JWTIssuer != values["JWT_ISSUER"] ||
		cfg.Security.JWTAudience != values["JWT_AUDIENCE"] || cfg.Security.DelegationSecret != values["PDP_SHARED_SECRET"] ||
		cfg.Security.DelegationActiveKeyID != "delegation-a" || !cfg.Security.DelegationKeyringExplicit ||
		cfg.Security.AuditActiveKeyID != "audit-a" || !cfg.Security.AuditKeyringExplicit ||
		cfg.Security.TLSCertFile != values["PDP_TLS_CERT"] || cfg.Security.TLSKeyFile != values["PDP_TLS_KEY"] || cfg.Security.TLSCAFile != values["PDP_TLS_CA"] {
		t.Fatalf("security environment binding mismatch: %+v", cfg.Security)
	}
}

func TestConfig_ProductionValidation(t *testing.T) {
	valid := map[string]string{
		"APP_ENV":                             "production",
		"DATABASE_URL":                        "postgres://policy:secret@db.internal:5432/policy_engine?sslmode=require",
		"JWT_SECRET":                          "production-jwt-secret-at-least-32-characters",
		"JWT_ISSUER":                          "https://identity.example.test",
		"JWT_AUDIENCE":                        "standalone-policy-engine",
		"PDP_DELEGATION_ACTIVE_KID":           "key-2026-09",
		"PDP_DELEGATION_KEYS_JSON":            `{"key-2026-08":"previous-production-delegation-secret-32-chars","key-2026-09":"active-production-delegation-secret-32-characters"}`,
		"PDP_TLS_CERT":                        "/run/secrets/pdp/tls.crt",
		"PDP_TLS_KEY":                         "/run/secrets/pdp/tls.key",
		"PDP_TLS_CA":                          "/run/secrets/pdp/ca.crt",
		"LOG_KEK_ACTIVE_KID":                  "audit-2026-09",
		"LOG_KEKS_JSON":                       `{"audit-2026-08":"test-audit-kek-old-32-bytes-key!","audit-2026-09":"test-audit-kek-new-32-bytes-key!"}`,
		"AUDIT_ARCHIVE_BUCKET":                "pdp-audit-archive-production-123",
		"AUDIT_ARCHIVE_PREFIX":                "pdp-audit",
		"AUDIT_ARCHIVE_REGION":                "ap-southeast-1",
		"AUDIT_ARCHIVE_EXPECTED_BUCKET_OWNER": "123456789012",
	}

	tests := []struct {
		name      string
		override  map[string]string
		wantError string
	}{
		{name: "valid production security", override: map[string]string{}},
		{name: "localhost database", override: map[string]string{"DATABASE_URL": "postgres://postgres:postgres@localhost:5432/db"}, wantError: "DATABASE_URL"},
		{name: "default JWT secret", override: map[string]string{"JWT_SECRET": developmentJWTSecret}, wantError: "JWT_SECRET"},
		{name: "short JWT secret", override: map[string]string{"JWT_SECRET": "short"}, wantError: "JWT_SECRET"},
		{name: "development issuer", override: map[string]string{"JWT_ISSUER": "standalone-policy-engine-dev"}, wantError: "JWT_ISSUER"},
		{name: "development audience", override: map[string]string{"JWT_AUDIENCE": "standalone-policy-engine-pdp"}, wantError: "JWT_AUDIENCE"},
		{name: "missing active delegation key", override: map[string]string{"PDP_DELEGATION_ACTIVE_KID": "missing"}, wantError: "PDP_DELEGATION_ACTIVE_KID"},
		{name: "short delegation key", override: map[string]string{"PDP_DELEGATION_KEYS_JSON": `{"key-2026-09":"short"}`}, wantError: "delegation active key"},
		{name: "missing TLS key", override: map[string]string{"PDP_TLS_KEY": " "}, wantError: "PDP_TLS"},
		{name: "missing audit active key", override: map[string]string{"LOG_KEK_ACTIVE_KID": "missing"}, wantError: "LOG_KEK_ACTIVE_KID"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range valid {
				t.Setenv(key, value)
			}
			for key, value := range test.override {
				t.Setenv(key, value)
			}
			_, err := Load()
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("expected valid production config, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected error containing %q, got %v", test.wantError, err)
			}
		})
	}
}

func TestConfigRejectsIncompleteAuditArchive(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "bucket only", env: map[string]string{"AUDIT_ARCHIVE_BUCKET": "archive"}},
		{name: "prefix without bucket", env: map[string]string{"AUDIT_ARCHIVE_PREFIX": "pdp-audit"}},
		{name: "unsafe prefix", env: map[string]string{"AUDIT_ARCHIVE_BUCKET": "archive", "AUDIT_ARCHIVE_PREFIX": "../audit", "AUDIT_ARCHIVE_REGION": "ap-southeast-1", "AUDIT_ARCHIVE_EXPECTED_BUCKET_OWNER": "123456789012"}},
		{name: "invalid owner", env: map[string]string{"AUDIT_ARCHIVE_BUCKET": "archive", "AUDIT_ARCHIVE_PREFIX": "pdp-audit", "AUDIT_ARCHIVE_REGION": "ap-southeast-1", "AUDIT_ARCHIVE_EXPECTED_BUCKET_OWNER": "not-an-account"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			if _, err := Load(); err == nil {
				t.Fatal("expected incomplete audit archive configuration to fail")
			}
		})
	}
}

func TestConfigRejectsPartialOrMalformedDelegationKeyring(t *testing.T) {
	tests := []struct {
		name   string
		active string
		keys   string
	}{
		{name: "active only", active: "key-a"},
		{name: "keys only", keys: `{"key-a":"delegation-secret-at-least-32-characters"}`},
		{name: "malformed JSON", active: "key-a", keys: `{`},
		{name: "active absent", active: "key-b", keys: `{"key-a":"delegation-secret-at-least-32-characters"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("PDP_DELEGATION_ACTIVE_KID", test.active)
			t.Setenv("PDP_DELEGATION_KEYS_JSON", test.keys)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid delegation key-ring configuration to fail")
			}
		})
	}
}

func TestConfigRejectsPartialOrMalformedAuditKeyring(t *testing.T) {
	tests := []struct {
		name   string
		active string
		keys   string
	}{
		{name: "active only", active: "audit-a"},
		{name: "keys only", keys: `{"audit-a":"test-audit-kek-old-32-bytes-key!"}`},
		{name: "malformed JSON", active: "audit-a", keys: `{`},
		{name: "active absent", active: "audit-b", keys: `{"audit-a":"test-audit-kek-old-32-bytes-key!"}`},
		{name: "wrong key length", active: "audit-a", keys: `{"audit-a":"short"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LOG_KEK_ACTIVE_KID", test.active)
			t.Setenv("LOG_KEKS_JSON", test.keys)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid audit key-ring configuration to fail")
			}
		})
	}
}
