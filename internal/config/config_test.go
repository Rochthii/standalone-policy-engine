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

func TestConfig_ProductionValidation(t *testing.T) {
	valid := map[string]string{
		"APP_ENV":                   "production",
		"DATABASE_URL":              "postgres://policy:secret@db.internal:5432/policy_engine?sslmode=require",
		"JWT_SECRET":                "production-jwt-secret-at-least-32-characters",
		"JWT_ISSUER":                "https://identity.example.test",
		"JWT_AUDIENCE":              "standalone-policy-engine",
		"PDP_DELEGATION_ACTIVE_KID": "key-2026-09",
		"PDP_DELEGATION_KEYS_JSON":  `{"key-2026-08":"previous-production-delegation-secret-32-chars","key-2026-09":"active-production-delegation-secret-32-characters"}`,
		"PDP_TLS_CERT":              "/run/secrets/pdp/tls.crt",
		"PDP_TLS_KEY":               "/run/secrets/pdp/tls.key",
		"PDP_TLS_CA":                "/run/secrets/pdp/ca.crt",
		"LOG_KEK_ACTIVE_KID":        "audit-2026-09",
		"LOG_KEKS_JSON":             `{"audit-2026-08":"test-audit-kek-old-32-bytes-key!","audit-2026-09":"test-audit-kek-new-32-bytes-key!"}`,
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
