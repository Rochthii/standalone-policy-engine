# Multi-Tenant Isolation Specification

Tài liệu này đặc tả cơ chế cô lập và bảo vệ dữ liệu khách thuê (Multi-Tenant Isolation) tại tầng logic, bộ nhớ RAM và tầng lưu trữ của **Standalone Policy Engine**.

---

## 1. Nguyên tắc Cô lập theo thiết kế (Isolation by Design)

Hệ thống cung cấp dịch vụ phân quyền chung cho nhiều Tenant (SaaS Model) nhưng phải đảm bảo không một Tenant nào có thể can thiệp hay nhìn thấy cấu trúc luật của Tenant khác.

Để làm được điều này, **`tenant_id`** được coi là một thuộc tính bắt buộc ở mọi cấp độ:

```text
[gRPC CheckRequest] ──► [Verify tenant_id in JWT claims] ──► [Lookup RAM Trie by tenant_id Key] ──► [Evaluate ASTs]
```

---

## 2. Các lớp Cô lập An toàn (Isolation Layers)

### A. Cô lập ở tầng Mạng (gRPC PEP Context validation)
*   API Gateway (PEP) khi nhận request từ người dùng sẽ thực hiện giải mã token xác thực (JWT) để trích xuất `tenant_id` từ claims bảo mật đã được ký số của OIDC/IdP.
*   PEP truyền `tenant_id` này vào request gRPC `CheckRequest`. PDP tuyệt đối không chấp nhận các request không có hoặc truyền `tenant_id` rỗng.

### B. Cô lập ở tầng Bộ nhớ (In-Memory Isolation)
*   Cây chỉ mục Radix Trie trên RAM sử dụng `tenant_id` làm khóa phân cấp cao nhất (Root-level Key).
*   Khi PDP thực hiện tra cứu, con trỏ Trie sẽ lập tức di chuyển đến nhánh của Tenant tương ứng:
    ```go
    tenantTrie, exists := trieRoot.Children[request.TenantID]
    ```
    Nếu `exists` là false, Engine kết luận ngay lập tức là `DENY` và dừng cuộc gọi. Không có bất kỳ thuật toán quét chéo (cross-trie scan) nào được phép tồn tại trên bộ nhớ.

### C. Cô lập ở tầng Cơ sở dữ liệu (Storage Isolation)
*   **PostgreSQL RLS (Row-Level Security):** Dù PDP là superuser kết nối database, các câu lệnh truy vấn nạp chính sách vẫn được bảo vệ bởi PostgreSQL RLS để tránh lỗi logic trong code của Control Plane làm rò rỉ dữ liệu.
*   Mỗi khi Control Plane thực hiện lưu/đọc chính sách, biến cấu hình session `app.current_tenant` phải được nạp động để PostgreSQL tự động lọc dữ liệu.
