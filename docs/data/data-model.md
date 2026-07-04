# Data Model & Storage Strategy Specification

Tài liệu này đặc tả chi tiết mô hình dữ liệu (Data Model), thiết kế schema lưu trữ bền vững (Schema Design) và chiến lược quản lý lưu trữ cho **Standalone Policy Engine**.

---

## 1. Chiến lược Lưu trữ (Storage Strategy)

Mặc dù Data Plane chạy hoàn toàn trên RAM để đảm bảo tốc độ tối đa, hệ thống vẫn bắt buộc phải có một tầng lưu trữ bền vững (Persistent Storage Layer) để nạp lại chính sách khi hệ thống restart hoặc khi nhân bản pod mới.

Chúng ta sử dụng **Mô hình Hybrid Storage**:
1.  **Central Persistent Storage (PostgreSQL):** Lưu trữ tập trung toàn bộ các chính sách, lịch sử phiên bản của chính sách, thông tin Tenant và Audit Logs. Đây là nguồn dữ liệu chuẩn (Single Source of Truth).
2.  **Edge Embedded Storage (BadgerDB - Tùy chọn cho Sidecar):** Trong kịch bản triển khai dạng Sidecar Daemon, mỗi PDP Pod sẽ tự chạy một BadgerDB cục bộ (Embedded Key-Value viết bằng Go thuần). Khi khởi động, PDP Sidecar nạp dữ liệu từ BadgerDB cục bộ ngay lập tức để phục vụ PEP mà không cần đợi kết nối mạng sang PostgreSQL trung tâm.

---

## 2. Thiết kế Schema Cơ sở dữ liệu (PostgreSQL Schema)

Dưới đây là các bảng dữ liệu cốt lõi lưu trữ trên PostgreSQL trung tâm:

```sql
-- Kích hoạt extension UUID để sinh ID ngẫu nhiên an toàn
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Bảng quản lý Tenant (Khách thuê)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, SUSPENDED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Bảng lưu trữ Chính sách phân quyền (Policies)
CREATE TABLE policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    effect VARCHAR(10) NOT NULL, -- ALLOW, DENY
    policy_text TEXT NOT NULL, -- Nội dung DSL thô (ví dụ: 'permit(principal == user:alice, ...)')
    ast_json JSONB NOT NULL, -- Cây AST đã biên dịch lưu dưới dạng JSONB để tra cứu nhanh
    version INT NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, DRAFT, SUSPENDED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tạo Index để tối ưu hóa việc nạp chính sách theo Tenant khi khởi động
CREATE INDEX idx_policies_tenant_status ON policies(tenant_id, status);

-- 3. Bảng kiểm toán quyết định (Decision Audit Logs) - Bảng WORM
CREATE TABLE decision_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    request_subject VARCHAR(255) NOT NULL,
    request_action VARCHAR(255) NOT NULL,
    request_resource VARCHAR(255) NOT NULL,
    decision VARCHAR(10) NOT NULL, -- ALLOW, DENY
    matched_policy_id UUID,
    evaluated_context JSONB,
    evaluated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tạo Index phục vụ việc truy xuất log kiểm toán của SOC Dashboard
CREATE INDEX idx_audit_logs_tenant_time ON decision_audit_logs(tenant_id, evaluated_at DESC);
```

---

## 3. Chính sách Lưu trữ & Sao lưu (Retention & Backup Policy)

### A. Ghi log kiểm toán (Decision Logs Retention)
*   Do lượng log quyết định sinh ra cực kỳ lớn (hàng triệu bản ghi mỗi ngày dưới tải cao), bảng `decision_audit_logs` trên PostgreSQL chỉ lưu trữ nóng trong vòng **`30 ngày`**.
*   Một tiến trình nền (Cronjob) chạy hàng đêm sẽ tự động đóng gói các log cũ hơn 30 ngày, xuất ra định dạng Parquet nén, đẩy lên **Cold Storage (AWS S3 Glacier / Google Cloud Archive Storage)** để lưu trữ dài hạn trong 2 năm phục vụ mục đích pháp lý, sau đó tự động xóa vĩnh viễn.

### B. Chiến lược Sao lưu (Backup Policy)
*   **Hourly Backup:** Thực hiện sao lưu gia tăng (Incremental Backup) tự động hàng giờ đối với bảng `tenants` và `policies` (dung lượng nhỏ nhưng cực kỳ quan trọng).
*   **Daily Full Backup:** Thực hiện sao lưu toàn phần (Full Database Backup) vào lúc 02:00 sáng hàng ngày. Bản sao lưu được mã hóa bằng AES-256 và nhân bản sang một Cloud Region khác để đảm bảo an toàn tuyệt đối trước thảm họa thiên tai (Disaster Recovery).
