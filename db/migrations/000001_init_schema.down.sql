-- Migration 000001 rollback: xóa toàn bộ bảng khởi tạo ban đầu.
-- CẢNH BÁO: Lệnh này sẽ xóa toàn bộ dữ liệu chính sách và tenant.

DROP INDEX IF EXISTS idx_policies_tenant_status;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS tenants;
