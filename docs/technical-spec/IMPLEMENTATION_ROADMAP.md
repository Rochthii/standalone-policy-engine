# IMPLEMENTATION_ROADMAP.md — Lộ Trình Phân Tử Hóa Triển Khai (Atomic Task Breakdown)

Tài liệu này chia nhỏ toàn bộ quá trình hiện thực hóa hệ thống thành các **nhiệm vụ nguyên tử (Atomic Micro-Tasks)**. Mỗi task có ranh giới mã nguồn độc lập, tiêu chí nghiệm thu rõ ràng (Acceptance Criteria), và lệnh kiểm thử tự động tương ứng (Verification Command) — **làm đến đâu, kiểm chứng đến đó, tuyệt đối không code mò**.

---

## THỨ TỰ THỰC THI CHUẨN (EXECUTION SEQUENCE)

```text
[BƯỚC 1: PROTO CONTRACT]
Cập nhật proto/v1/policy.proto ──► Compile stub sang Go & Python
                                         │
[BƯỚC 2: GO BACKEND]                     ▼
Viết delegation.go ──► Sửa grpc_server.go ──► Nạp configs/policies.cedar
                                         │
[BƯỚC 3: GO UNIT TEST]                   ▼
Chạy: go test -v -race ./internal/security ./internal/server (Pass 100%)
                                         │
[BƯỚC 4: ODOO PEP ADDON]                 ▼
Viết pdp_client.py ──► delegation_grant.py ──► purchase_order.py ──► __manifest__.py
                                         │
[BƯỚC 5: E2E INTEGRATION]                ▼
Khởi động cụm Docker/Local ──► Chạy 7 Test Vectors kiểm chứng
```

---

## QUY ƯỚC DỮ LIỆU BẮT BUỘC (DATA & CRYPTO CONVENTIONS)

### 1. Master/Tenant Shared Secret
* Go PDP là Stateless, không lưu trữ từng Grant trong CSDL.
* Secret dùng để tính toán và xác thực HMAC là **Master Secret** được cấu hình qua biến môi trường:
  * `PDP_SHARED_SECRET` (Mặc định dev: `"pdp_master_secret_key_32bytes!"`).

### 2. Chuỗi Nối Chuẩn Hóa (Canonical String Formula)
Cả Odoo (Python) và Go PDP bắt buộc sử dụng chung một công thức ghép chuỗi trước khi băm HMAC-SHA256:

$$\text{Payload} = \text{grant\_id} \parallel \text{delegator} \parallel \text{agent} \parallel \text{amount} \parallel \text{valid\_until}$$

* **Quy ước định dạng:**
  `fmt.Sprintf("%s|%s|%s|%s|%s", grantID, delegator, agent, amount, validUntil)`
* **Ví dụ cụ thể:**
  `"42|user:bob|agent:procurement_copilot|1500|1798761600"`

---

## CHI TIẾT CÁC BƯỚC THỰC THI (ATOMIC MICRO-TASKS)

### BƯỚC 1: HỢP ĐỒNG GIAO THỨC PROTOBUF (`proto/v1/policy.proto`)
* **Files:** `proto/v1/policy.proto`
* **Micro-tasks:**
  * [ ] **1.1:** Mở rộng `proto/v1/policy.proto`:
    ```protobuf
    message RevokeRequest {
      string tenant_id = 1;
      string grant_id = 2;
      string revoked_by = 3;
      string reason = 4;
    }

    message RevokeResponse {
      bool success = 1;
      int64 revoked_at = 2;
      string message = 3;
    }

    service PolicyDecisionPoint {
      rpc CheckAccess (CheckAccessRequest) returns (CheckAccessResponse);
      rpc ExplainDecision (ExplainRequest) returns (ExplainResponse);
      rpc RevokeDelegation (RevokeRequest) returns (RevokeResponse);
    }
    ```
  * [ ] **1.2:** Biên dịch Go Stub:
    `protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/v1/policy.proto`
  * [ ] **1.3:** Biên dịch Python Stub cho Odoo:
    `python -m grpc_tools.protoc -I. --python_out=../ERP_Mastery_Hub/02_Project_2_Odoo_Go_PDP_Approval/custom_addons/pdp_authorizer/proto --grpc_python_out=../ERP_Mastery_Hub/02_Project_2_Odoo_Go_PDP_Approval/custom_addons/pdp_authorizer/proto proto/v1/policy.proto`
* **Nghiệm thu:** Cả Go và Python stubs đều được sinh ra đầy đủ phương thức `RevokeDelegation`.

---

### BƯỚC 2: HIỆN THỰC HÓA GO PDP BACKEND
* **Files:** `internal/security/delegation.go`, `internal/server/grpc_server.go`, `configs/policies.cedar`
* **Micro-tasks:**
  * [ ] **2.1: `internal/security/delegation.go`:**
    * Struct `DelegationManager` với `revocationMap sync.Map` (lưu `grant_id -> revoked_at`).
    * Hàm `Revoke(grantID string) int64`.
    * Hàm `IsRevoked(grantID string) bool` ($O(1)$ trên RAM, độ trễ $< 50$ ns).
    * Hàm `VerifyProof(secret, grantID, delegator, agent, amount, validUntil, proof string) bool` dùng `hmac.Equal`.
  * [ ] **2.2: `internal/server/grpc_server.go`:**
    * Tích hợp `DelegationManager` vào `GRPCServer`.
    * Implement RPC `RevokeDelegation`: gọi `delegationMgr.Revoke(req.GrantId)` và trả về `RevokeResponse`.
    * Hook đầu hàm `CheckAccess`:
      * Kiểm tra `req.Context["delegation_grant_id"]`: Nếu đã bị Revoke $\to$ từ chối ngay lập tức (`Decision_DENY`, triệt tiêu TOCTOU).
      * Kiểm tra `req.Context["delegation_proof"]`: Nếu có chuỗi ủy quyền, đối soát HMAC Canonical String bằng `PDP_SHARED_SECRET`. Nếu không khớp $\to$ trả `codes.PermissionDenied`.
  * [ ] **2.3: `configs/policies.cedar`:**
    * Đóng gói 6 luật P2P chuẩn, khóa cứng toán tử `contains` cho SoD (`context.delegation_chain contains resource.creator_id`).

---

### BƯỚC 3: KIỂM THỬ ĐƠN VỊ GO (GO UNIT & RACE TEST)
* **Files:** `internal/security/delegation_test.go`, `tests/delegation_interceptor_test.go`
* **Micro-tasks:**
  * [ ] **3.1:** Viết test kiểm tra tính đúng đắn của HMAC Canonical String: Khớp mã $\to$ true; sai 1 ký tự $\to$ false.
  * [ ] **3.2:** Viết test TOCTOU: Gửi `RevokeDelegation` $\to$ kiểm tra `CheckAccess` ngay lập tức bị từ chối trong $< 1\,\mu\text{s}$.
  * [ ] **3.3:** Chạy toàn bộ test với cờ race detector:
    `go test -v -race ./internal/security ./internal/server`
* **Nghiệm thu:** 100% tests PASS, 0 data race.

---

### BƯỚC 4: HIỆN THỰC HÓA CUSTOM MODULE ODOO 17 PEP
* **Files:** `custom_addons/pdp_authorizer/`
* **Micro-tasks:**
  * [ ] **4.1: `models/pdp_client.py`:**
    * Singleton `SafePDPClient` kiểm tra `os.getpid()` tự động re-init channel sau khi worker fork.
    * Gọi stub `CheckAccess` và `RevokeDelegation`.
    * Cấu hình Fail-Closed an toàn khi có lỗi kết nối.
  * [ ] **4.2: `models/delegation_grant.py` (`pdp.delegation.grant`):**
    * Model quản lý: `user_id`, `agent_id`, `max_amount`, `valid_from`, `valid_until`, `state`.
    * Phương thức `generate_proof(amount)` ghép đúng chuỗi chuẩn hóa:
      `f"{self.id}|{self.user_id.login}|{self.agent_id}|{int(amount)}|{int(self.valid_until.timestamp())}"`
    * Phương thức `action_revoke()`: đổi `state = 'revoked'` và gọi `client.revoke_delegation(grant_id)`.
  * [ ] **4.3: `models/purchase_order.py`:**
    * Thêm liên kết `delegation_grant_id`, `ai_agent_id`, `delegated_by_id`, `pdp_status`.
    * Override `button_confirm()`: đóng gói payload chuẩn gRPC.
    * **Non-Rollback Handler:** Khi nhận `REQUIRE_HUMAN_APPROVAL`, chuyển `state = 'to approve'`, gọi `activity_schedule(...)`, `return True` (không raise UserError).
  * [ ] **4.4: `__manifest__.py` & Phân quyền:**
    * Thêm `external_dependencies: {'python': ['grpcio', 'protobuf']}`.
    * Đăng ký views và quyền CSV trong `security/ir.model.access.csv`.

---

### BƯỚC 5: TÍCH HỢP TOÀN TRÌNH E2E & ĐO ĐẠC HỌC THUẬT
* **Micro-tasks:**
  * [ ] **5.1:** Khởi động hệ thống kết nối Go PDP (:50051) và Odoo (:8069).
  * [ ] **5.2:** Chạy 7 Test Vectors (S1 $\to$ S7) kiểm chứng toàn bộ luồng nghiệp vụ.
  * [ ] **5.3:** Đo độ trễ E2E, chụp ảnh minh chứng Odoo Non-Rollback đưa vào Thuyết minh Đồ án.
