# Decision API Specification

Tài liệu này đặc tả chi tiết giao tiếp (API) của luồng quyết định phân quyền (Data Plane) trong **Standalone Policy Engine**.

---

## 1. gRPC Contract: CheckAccess

Đây là API Hot Path chính của hệ thống, được gọi từ các API Gateways/PEPs.

*   **Service Name:** `policy.PolicyDecisionPoint`
*   **Method Name:** `CheckAccess`

### Protobuf Messages:

```protobuf
syntax = "proto3";

package policy.v1;

option go_package = "standalone-policy-engine/proto/v1;policyv1";

service PolicyDecisionPoint {
  // Đánh giá quyền truy cập dựa trên ngữ cảnh động
  rpc CheckAccess (CheckAccessRequest) returns (CheckAccessResponse);
  
  // Đánh giá quyền kèm giải thích chi tiết luật áp dụng
  rpc ExplainDecision (ExplainRequest) returns (ExplainResponse);
}

message CheckAccessRequest {
  string tenant_id = 1;         // Khóa đa khách thuê
  string subject = 2;           // Định danh người dùng (user:alice hoặc role:admin)
  string action = 3;            // Hành động thực hiện (action:READ)
  string resource = 4;          // Tài nguyên tác động (file:doc.pdf)
  map<string, string> context = 5; // Ngữ cảnh động (ip="10.0.0.1", time="2026-07-04T10:20:00Z")
}

message CheckAccessResponse {
  enum Decision {
    DENY = 0;
    ALLOW = 1;
  }
  Decision decision = 1;        // Quyết định ALLOW hoặc DENY
  string matched_policy_id = 2; // ID chính sách quyết định (ALLOW hoặc FORBID)
}
```

---

## 2. REST HTTP Fallback Contract: POST /decisions

Mặc dù khuyên dùng gRPC cho môi trường microservices nội bộ, PDP vẫn cung cấp một HTTP endpoint fallback hỗ trợ REST clients.

*   **Method:** `POST`
*   **Path:** `/api/v1/decisions`
*   **Content-Type:** `application/json`

### Request Body:
```json
{
  "tenant_id": "1111-1111",
  "subject": "user:alice",
  "action": "READ",
  "resource": "file:doc.pdf",
  "context": {
    "ip": "10.0.0.1",
    "request_time": "2026-07-04T10:20:00Z"
  }
}
```

### Response Body (200 OK):
```json
{
  "decision": "ALLOW",
  "matched_policy_id": "4c0e395c-9c7b-4df2-8c19-9ef9892c556b",
  "evaluated_at": "2026-07-04T10:20:01Z"
}
```
