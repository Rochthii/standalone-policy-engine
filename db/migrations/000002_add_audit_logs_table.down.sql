-- Migration 000002 rollback: xóa bảng decision_audit_logs.

DROP INDEX IF EXISTS idx_audit_logs_tenant_time;
DROP TABLE IF EXISTS decision_audit_logs;
