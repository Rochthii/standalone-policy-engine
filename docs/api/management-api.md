# Policy Management API Specification

Tài liệu này đặc tả chi tiết giao diện REST HTTP API của **Control Plane** phục vụ công tác quản lý chính sách (Policy Lifecycle Management).

---

## 1. Danh sách các API Endpoint Quản trị

Mọi API quản trị bắt buộc phải được bảo vệ bằng JWT token trong header `Authorization: Bearer <token>`.

| Method | Path | Mô tả |
| :--- | :--- | :--- |
| `POST` | `/api/v1/tenants/{tenant_id}/policies` | Tạo mới chính sách dạng bản thảo (DRAFT) |
| `PUT` | `/api/v1/tenants/{tenant_id}/policies/{policy_id}` | Cập nhật nội dung chính sách |
| `POST` | `/api/v1/tenants/{tenant_id}/policies/{policy_id}/publish` | Phát hành (Kích hoạt) chính sách lên RAM |
| `GET` | `/api/v1/tenants/{tenant_id}/policies/{policy_id}/history` | Xem lịch sử các phiên bản (Policy Versioning) |
| `DELETE` | `/api/v1/tenants/{tenant_id}/policies/{policy_id}` | Xóa bỏ chính sách |

---

## 2. Đặc tả Chi tiết: Phát hành Chính sách (Publish Policy)

Khi chính sách được tạo mới ở trạng thái DRAFT, nó chưa được nạp lên RAM của Data Plane. Để kích hoạt, quản trị viên gọi API Publish:

*   **Method:** `POST`
*   **Path:** `/api/v1/tenants/1111-1111/policies/4c0e395c-9c7b-4df2-8c19-9ef9892c556b/publish`

### Response (200 OK):
```json
{
  "policy_id": "4c0e395c-9c7b-4df2-8c19-9ef9892c556b",
  "status": "ACTIVE",
  "published_version": 2,
  "published_at": "2026-07-04T10:24:00Z",
  "ast_nodes_count": 8,
  "sync_event_sent": true
}
```

### Cơ chế hoạt động nền:
*   API này sẽ đánh dấu chính sách là `ACTIVE` trong PostgreSQL.
*   Control Plane phát đi sự kiện `PolicyUpdatedEvent` qua Redis Pub/Sub.
*   Các PDP instance nhận được sự kiện, nạp AST vào bộ nhớ RAM trie, hoàn tất cập nhật nóng không downtime.
