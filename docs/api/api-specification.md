# API & gRPC Contract Specification

Tài liệu này đặc tả chi tiết giao diện bắt tay gRPC (Data Plane) và REST HTTP API (Control Plane) của **Standalone Policy Engine**, bao gồm cả thiết kế mô hình lỗi và chiến lược phiên bản.

---

## 1. gRPC API Contract (Data Plane)

Đây là giao diện tốc độ cao phục vụ PEP gọi sang PDP để kiểm tra quyền.

### Protobuf Definition (`proto/policy.proto`):

```protobuf
syntax = "proto3";

package policy;

option go_package = "standalone-policy-engine/proto;policy";

// Dịch vụ PDP (Policy Decision Point)
service PolicyDecisionPoint {
  // Kiểm tra quyền truy cập dựa trên ngữ cảnh động
  rpc CheckPermission (CheckRequest) returns (CheckResponse);
}

// Yêu cầu kiểm tra quyền
message CheckRequest {
  string tenant_id = 1;    // Định danh khách thuê để cách ly dữ liệu
  string subject = 2;      // Đối tượng yêu cầu (ví dụ: "user:alice", "role:admin")
  string action = 3;       // Hành động (ví dụ: "READ", "DELETE")
  string resource = 4;     // Tài nguyên tác động (ví dụ: "file:report.pdf")
  map<string, string> context = 5; // Ngữ cảnh động (ví dụ: ip="10.0.0.5", time="14:30")
}

// Phản hồi kết quả quyết định
message CheckResponse {
  enum Decision {
    DENY = 0;
    ALLOW = 1;
  }
  Decision decision = 1;   // Quyết định cuối cùng (ALLOW hoặc DENY)
  string reason = 2;       // Lý do đưa ra quyết định (luật nào được áp dụng)
  string evaluated_at = 3; // Timestamp thời điểm đánh giá (ISO 8601)
}
```

---

## 2. REST HTTP API Contract (Control Plane)

API này phục vụ hệ thống Admin quản trị chính sách (CRUD).

### A. Tạo mới chính sách (Create Policy)
*   **Method:** `POST`
*   **Path:** `/api/v1/tenants/{tenant_id}/policies`
*   **Headers:** `Content-Type: application/json`, `Authorization: Bearer <token>`
*   **Request Body:**
    ```json
    {
      "effect": "ALLOW",
      "policy_text": "permit(principal == user:alice, action == action:DELETE, resource == file:doc.pdf) when { context.ip in \"192.168.1.0/24\" };"
    }
    ```
*   **Response (201 Created):**
    ```json
    {
      "id": "4c0e395c-9c7b-4df2-8c19-9ef9892c556b",
      "tenant_id": "11111111-1111-1111-1111-111111111111",
      "effect": "ALLOW",
      "policy_text": "permit(principal == user:alice, ...)",
      "version": 1,
      "status": "ACTIVE",
      "created_at": "2026-07-04T10:20:00Z"
    }
    ```

### B. Xóa chính sách (Delete Policy)
*   **Method:** `DELETE`
*   **Path:** `/api/v1/tenants/{tenant_id}/policies/{policy_id}`
*   **Response (204 No Content):** Thành công, chính sách bị xóa khỏi DB và RAM cache lập tức.

---

## 3. Mô hình Lỗi (Error Model)

Hệ thống tuân thủ mô hình lỗi chuẩn của gRPC và REST:

### Mã lỗi gRPC thường gặp:
*   **`INVALID_ARGUMENT (3)`:** Đầu vào thiếu `tenant_id`, `subject` hoặc `resource`.
*   **`UNAUTHENTICATED (16)`:** Chứng chỉ mTLS không hợp lệ hoặc thiếu.
*   **`INTERNAL (13)`:** Lỗi hệ thống nội bộ (ví dụ: RAM index bị hỏng, lỗi DB).

### Định dạng lỗi REST API (RFC 7807 - Problem Details):
```json
{
  "type": "https://policy-engine.internal/errors/invalid-policy-syntax",
  "title": "Syntax Error in Policy DSL",
  "status": 400,
  "detail": "Failed to parse DSL: Line 1, Col 42: expected 'when', got 'where'",
  "instance": "/api/v1/tenants/1111-1111/policies",
  "invalid_params": [
    {
      "name": "policy_text",
      "reason": "Mismatched token at position 42"
    }
  ]
}
```

---

## 4. Chiến lược Phiên bản (Versioning Strategy)

*   **gRPC Versioning:** Được quản lý trực tiếp bằng package Protobuf: `package policy.v1;`. Khi có thay đổi đột biến không tương thích ngược (Breaking change), ta sẽ định nghĩa file proto mới tại `proto/v2/policy.proto` chạy song song trên cổng mới hoặc multiplexing cùng cổng.
*   **REST API Versioning:** Đặt phiên bản trực tiếp trên URL path: `/api/v1/...`. Phiên bản cũ v1 sẽ được duy trì tối thiểu 12 tháng kể từ ngày phát hành v2 để các service khác kịp chuyển đổi.
