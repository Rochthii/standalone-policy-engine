-- Migration 000003: Add monotonic revision column to tenants table for distributed sync.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 1;
CREATE INDEX IF NOT EXISTS idx_tenants_revision ON tenants(id, revision);