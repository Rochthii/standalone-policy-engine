-- Persist tenant-scoped role DAG edges so every PDP replica builds the same closure.
CREATE TABLE IF NOT EXISTS role_inheritances (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    parent     VARCHAR(255) NOT NULL,
    child      VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, parent, child),
    CONSTRAINT role_inheritances_no_self_edge CHECK (parent <> child)
);

CREATE INDEX IF NOT EXISTS idx_role_inheritances_tenant
    ON role_inheritances(tenant_id);
