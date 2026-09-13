DROP INDEX IF EXISTS idx_audit_logs_request_id;
DROP INDEX IF EXISTS idx_audit_logs_audit_id;

ALTER TABLE decision_audit_logs
    DROP COLUMN IF EXISTS integrity_tag,
    DROP COLUMN IF EXISTS trace_id,
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS key_id,
    DROP COLUMN IF EXISTS payload_version,
    DROP COLUMN IF EXISTS revision_id,
    DROP COLUMN IF EXISTS event_timestamp_ns,
    DROP COLUMN IF EXISTS audit_id;
