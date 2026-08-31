DROP INDEX IF EXISTS idx_tenants_revision;
ALTER TABLE tenants DROP COLUMN IF EXISTS revision;