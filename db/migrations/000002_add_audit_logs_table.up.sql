-- Migration 000002: Add decision_audit_logs table.
-- Extracted from inline initSchema() in internal/storage/postgres.go (Sprint 6).

CREATE TABLE IF NOT EXISTS decision_audit_logs (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         UUID NOT NULL,
    request_subject   VARCHAR(255),
    request_action    VARCHAR(255),
    request_resource  VARCHAR(255),
    decision          VARCHAR(10) NOT NULL,
    matched_policy_id UUID,
    evaluated_context JSONB,
    is_encrypted      BOOLEAN DEFAULT FALSE,
    encrypted_dek     TEXT,
    encrypted_payload TEXT,
    evaluated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_time ON decision_audit_logs(tenant_id, evaluated_at DESC);
