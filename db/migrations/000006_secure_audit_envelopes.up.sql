ALTER TABLE decision_audit_logs
    ADD COLUMN audit_id UUID,
    ADD COLUMN event_timestamp_ns BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN revision_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN payload_version SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN key_id TEXT,
    ADD COLUMN request_id TEXT,
    ADD COLUMN trace_id TEXT,
    ADD COLUMN integrity_tag TEXT;

UPDATE decision_audit_logs SET audit_id = gen_random_uuid() WHERE audit_id IS NULL;
UPDATE decision_audit_logs SET is_encrypted = FALSE WHERE is_encrypted IS NULL;

ALTER TABLE decision_audit_logs
    ALTER COLUMN audit_id SET NOT NULL,
    ALTER COLUMN audit_id SET DEFAULT gen_random_uuid(),
    ALTER COLUMN is_encrypted SET NOT NULL;

CREATE UNIQUE INDEX idx_audit_logs_audit_id ON decision_audit_logs(audit_id);
CREATE INDEX idx_audit_logs_request_id ON decision_audit_logs(request_id) WHERE request_id IS NOT NULL;
