CREATE TABLE IF NOT EXISTS delegation_revocations (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    grant_id VARCHAR(255) NOT NULL,
    revoked_by VARCHAR(255) NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (tenant_id, grant_id),
    CONSTRAINT delegation_revocations_valid_window CHECK (expires_at > revoked_at)
);

CREATE INDEX IF NOT EXISTS idx_delegation_revocations_active
    ON delegation_revocations (expires_at);
