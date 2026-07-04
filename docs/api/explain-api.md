# Explain API Specification

Tài liệu này đặc tả giao tiếp gRPC và REST HTTP hỗ trợ giải thích lý do quyết định phân quyền (Explain API) phục vụ SOC Dashboard và thanh tra hệ thống.

---

## 1. gRPC Contract: ExplainDecision

*   **Service Name:** `policy.PolicyDecisionPoint`
*   **Method Name:** `ExplainDecision`

### Protobuf Messages:

```protobuf
syntax = "proto3";

package policy.v1;

option go_package = "standalone-policy-engine/proto/v1;policyv1";

message ExplainRequest {
  string tenant_id = 1;
  string subject = 2;
  string action = 3;
  string resource = 4;
  map<string, string> context = 5;
}

message ExplainResponse {
  enum Decision {
    DENY = 0;
    ALLOW = 1;
  }
  Decision decision = 1;                // Quyết định cuối cùng
  string final_reason = 2;              // Giải thích tổng quát
  repeated PolicyMetadata matched = 3; // Danh sách các chính sách đã khớp điều kiện
}

message PolicyMetadata {
  string policy_id = 1;
  string effect = 2;                    // "permit" hoặc "forbid"
  string policy_text = 3;               // Text DSL thô của luật
}
```

---

## 2. REST HTTP Contract: POST /decisions/explain

*   **Method:** `POST`
*   **Path:** `/api/v1/decisions/explain`
*   **Content-Type:** `application/json`

### Response Body (200 OK):
```json
{
  "decision": "DENY",
  "final_reason": "Access explicitly forbidden by policy [4c0e395c-9c7b-4df2-8c19-9ef9892c556b] overrides allow rules.",
  "matched_policies": [
    {
      "id": "4c0e395c-9c7b-4df2-8c19-9ef9892c556b",
      "effect": "forbid",
      "policy_text": "forbid(principal in role:employee, action == any, resource == folder:classified_finance) when { context.device_status == \"compromised\" };"
    },
    {
      "id": "8e3c311a-9c7b-4df2-8c19-9ef9892c556c",
      "effect": "permit",
      "policy_text": "permit(principal in role:employee, action == action:READ, resource == folder:classified_finance);"
    }
  ]
}
```
*Giao diện SOC Dashboard sẽ sử dụng API này để render luồng giải thích quyết định một cách trực quan cho điều hành viên.*
