# IMPLEMENTATION_ROADMAP.md — Lộ Trình Phân Tử Hóa Triển Khai (Atomic Task Breakdown)

Tài liệu này chia nhỏ toàn bộ quá trình hiện thực hóa hệ thống thành các **nhiệm vụ nguyên tử (Atomic Micro-Tasks)**. Mỗi task có ranh giới mã nguồn độc lập, tiêu chí nghiệm thu rõ ràng (Acceptance Criteria), và lệnh kiểm thử tự động tương ứng (Verification Command) — **làm đến đâu, kiểm chứng đến đó, tuyệt đối không code mò**.

---

## TỔNG QUAN 3 PHA VÀ TIẾN ĐỘ THỰC HIỆN

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ PHA 1: HOÀN THIỆN TẦNG BIÊN GO PDP ENGINE (STANDALONE-POLICY-ENGINE)        │
│ [Task 1.1] Module HMAC & Revocation Map RAM O(1)                            │
│ [Task 1.2] Mở rộng Protobuf gRPC Contract (RevokeDelegation RPC)             │
│ [Task 1.3] Tầng 1 gRPC Security Interceptor & Blacklist Tra Cứu              │
│ [Task 1.4] Tệp hạt giống chính sách chuẩn hóa configs/policies.cedar        │
├─────────────────────────────────────────────────────────────────────────────┤
│ PHA 2: HOÀN THIỆN CUSTOM MODULE ODOO 17 PEP (PDP_AUTHORIZER)                │
│ [Task 2.1] Client gRPC PID-Safe & Hỗ trợ Gevent (models/pdp_client.py)       │
│ [Task 2.2] Model Ủy quyền & Sinh mã băm (models/delegation_grant.py)        │
│ [Task 2.3] Kế thừa purchase.order & Máy trạng thái Non-Rollback PEP         │
│ [Task 2.4] Phân quyền CSDL & Giao diện Odoo (ir.model.access.csv, views)     │
├─────────────────────────────────────────────────────────────────────────────┤
│ PHA 3: TÍCH HỢP TOÀN TRÌNH E2E & ĐO ĐẠC HỌC THUẬT (E2E & VERIFICATION)       │
│ [Task 3.1] Khởi động môi trường Docker Compose (Go PDP + Postgres + Odoo 17) │
│ [Task 3.2] Tự động hóa kiểm thử 7 Test Vectors (S1 -> S7)                   │
│ [Task 3.3] Xuất số liệu đo độ trễ E2E & Chụp ảnh minh chứng Thuyết minh      │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## PHA 1: HOÀN THIỆN TẦNG BIÊN GO PDP ENGINE (`standalone-policy-engine`)

### Task 1.1: Cấu trúc Dữ liệu `DelegationManager` (HMAC & Revocation Map)
* **File mục tiêu:** `internal/security/delegation.go` & `internal/security/delegation_test.go`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **1.1.1:** Khai báo struct `DelegationManager` với trường `revocationMap sync.Map` lưu `session_id string -> revoked_at int64`.
  * [ ] **1.1.2:** Viết hàm `RevokeSession(sessionID string)`: Ghi nhận thời điểm thu hồi vào `sync.Map` với độ trễ $< 1.0\,\mu\text{s}$.
  * [ ] **1.1.3:** Viết hàm `IsRevoked(sessionID string) bool`: Tra cứu $O(1)$ trên RAM không lock.
  * [ ] **1.1.4:** Viết hàm `VerifyProof(secret, delegator, agent, tool, amount, ts, proof string) bool`:
    * Ghép payload: `delegator + "|" + agent + "|" + tool + "|" + amount + "|" + ts`.
    * Tính HMAC-SHA256 và so khớp bằng `hmac.Equal` (chống timing-attack).
* **Tiêu chuẩn nghiệm thu (Acceptance Criteria):**
  * Chạy `go test -v -race ./internal/security`: PASS 100%, 0 data race.
  * Benchmark hàm `IsRevoked` đạt $< 50$ ns/op trên RAM.

---

### Task 1.2: Mở Rộng Hợp Đồng Protobuf (`proto/v1/policy.proto`)
* **File mục tiêu:** `proto/v1/policy.proto` & code gen `proto/v1/policy.pb.go`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **1.2.1:** Khai báo message `RevokeRequest { string tenant_id = 1; string session_id = 2; string revoked_by = 3; }`.
  * [ ] **1.2.2:** Khai báo message `RevokeResponse { bool success = 1; string message = 2; }`.
  * [ ] **1.2.3:** Thêm RPC `RevokeDelegation (RevokeRequest) returns (RevokeResponse);` vào `service PolicyDecisionPoint`.
  * [ ] **1.2.4:** Chạy lệnh sinh code: `protoc --go_out=. --go-grpc_out=. proto/v1/policy.proto`.
* **Tiêu chuẩn nghiệm thu:**
  * Build thành công thư viện protobuf Go, không lỗi import.

---

### Task 1.3: Tầng 1 gRPC Security Interceptor Trong `GRPCServer`
* **File mục tiêu:** `internal/server/grpc_server.go` & `internal/server/grpc_server_test.go`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **1.3.1:** Tích hợp con trỏ `delegationMgr *security.DelegationManager` vào `GRPCServer`.
  * [ ] **1.3.2:** Thêm chốt kiểm tra tại đầu hàm `CheckAccess`:
    * Nếu `context["delegation_grant_id"]` có trong `RevocationMap` $\to$ Trả về `Decision_DENY` ngay (triệt tiêu TOCTOU).
  * [ ] **1.3.3:** Thêm chốt kiểm tra HMAC tại đầu hàm `CheckAccess`:
    * Nếu có `context["delegation_proof"]`: Gọi `VerifyProof`. Nếu sai lệch $\to$ Báo lỗi `codes.PermissionDenied`.
  * [ ] **1.3.4:** Hiện thực hóa handler `RevokeDelegation(ctx, req)` nạp session vào `DelegationManager`.
* **Tiêu chuẩn nghiệm thu:**
  * Unit test chứng minh: Request mang session đã revoke bị chặn ngay trong $< 1.5\,\mu\text{s}$, không đi vào Hot-Path Core.

---

### Task 1.4: Tệp Hạt Giống Chính Sách Chuẩn (`configs/policies.cedar`)
* **File mục tiêu:** `configs/policies.cedar`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **1.4.1:** Tạo file `configs/policies.cedar` chứa đủ 6 luật P2P theo đúng đặc tả `POLICY_DSL_SPEC.md`.
  * [ ] **1.4.2:** Khóa cứng toán tử **`contains`** cho luật SoD: `context.delegation_chain contains resource.creator_id`.
  * [ ] **1.4.3:** Viết hàm nạp file `policies.cedar` tự động vào Engine khi `pdp-server` khởi động.
* **Tiêu chuẩn nghiệm thu:**
  * Server khởi động và log ra: `Loaded 6 policies from configs/policies.cedar successfully`.

---

## PHA 2: HOÀN THIỆN CUSTOM MODULE ODOO 17 PEP (`custom_addons/pdp_authorizer`)

### Task 2.1: PID-Safe gRPC Client Cho Odoo (`models/pdp_client.py`)
* **File mục tiêu:** `custom_addons/pdp_authorizer/models/pdp_client.py`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **2.1.1:** Kiểm tra môi trường `gevent` và khởi tạo `grpc.experimental.gevent.init_gevent()`.
  * [ ] **2.1.2:** Xây dựng class `SafePDPClient` lưu `self._pid = os.getpid()` và con trỏ kênh gRPC.
  * [ ] **2.1.3:** Viết hàm `get_stub()`: So sánh `os.getpid() != self._pid` để re-init channel nếu worker bị fork.
  * [ ] **2.1.4:** Viết phương thức `check_access(req_data)` với timeout 350ms và khối `except` Fail-Closed.
  * [ ] **2.1.5:** Viết phương thức `revoke_delegation(tenant_id, session_id, revoked_by)` gọi RPC sang Go PDP.
* **Tiêu chuẩn nghiệm thu:**
  * Chạy test trên Python 3.10+: Gọi thành công `check_access()` sang cổng `:50051`.

---

### Task 2.2: Model Ủy Quyền AI `pdp.delegation.grant` (`models/delegation_grant.py`)
* **File mục tiêu:** `custom_addons/pdp_authorizer/models/delegation_grant.py`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **2.2.1:** Định nghĩa model `pdp.delegation.grant` với các trường: `user_id`, `agent_id`, `max_amount`, `valid_from`, `valid_until`, `state`, `shared_secret`.
  * [ ] **2.2.2:** Viết phương thức `generate_proof(tool_name, amount)` sinh chuỗi HMAC-SHA256:
    * `payload = f"{self.user_id.login}|{self.agent_id}|{tool_name}|{int(amount)}|{int(time.time())}"`
  * [ ] **2.2.3:** Viết action button `action_revoke()`: Đổi `state = 'revoked'` và gọi `SafePDPClient.revoke_delegation()`.
* **Tiêu chuẩn nghiệm thu:**
  * Tạo bản ghi trên Odoo, bấm nút Revoke $\to$ Go PDP nhận tín hiệu và lưu vào `RevocationMap` RAM ngay lập tức.

---

### Task 2.3: Kế Thừa `purchase.order` & Cơ Chế Non-Rollback PEP
* **File mục tiêu:** `custom_addons/pdp_authorizer/models/purchase_order.py`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **2.3.1:** Thêm các trường: `pdp_status` (Selection), `ai_agent_id`, `delegated_by_id`, `delegation_grant_id`.
  * [ ] **2.3.2:** Override `button_confirm()`: Loại bỏ `requests.post` HTTP; chuyển sang gọi `SafePDPClient.check_access()`.
  * [ ] **2.3.3:** Đóng gói context: `"amount": str(int(order.amount_total))`, `"resource.creator_id"`, `"context.delegation_chain"`.
  * [ ] **2.3.4:** **Xử lý dứt điểm Odoo Rollback Trap:**
    * Nếu nhận `ALLOW` $\to$ Cập nhật `pdp_status = 'allow'` $\to$ gọi `super().button_confirm()`.
    * Nếu nhận `REQUIRE_HUMAN_APPROVAL` $\to$ Cập nhật `state = 'to approve'`, `pdp_status = 'require_approval'`, tạo `activity_schedule(...)` và `return True` (**TUYỆT ĐỐI KHÔNG raise UserError**).
    * Nếu nhận `Hard DENY` $\to$ Cập nhật `pdp_status = 'deny'` $\to$ `raise AccessError(...)` để rollback giao dịch.
* **Tiêu chuẩn nghiệm thu:**
  * Đơn hàng vượt hạn mức \$2,000 chuyển sang `'to approve'`, bản ghi được lưu an toàn trong PostgreSQL, xuất hiện thông báo việc cần làm trên giao diện Odoo.

---

### Task 2.4: Đóng Gói Manifest, Security & Giao Diện Odoo
* **File mục tiêu:** `custom_addons/pdp_authorizer/__manifest__.py`, `security/ir.model.access.csv`, `views/`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **2.4.1:** Cập nhật `__manifest__.py` khai báo dependencies `['purchase', 'mail']`.
  * [ ] **2.4.2:** Thêm quyền truy cập `security/ir.model.access.csv` cho model `pdp.delegation.grant`.
  * [ ] **2.4.3:** Tạo form view & tree view cho `pdp.delegation.grant` hiển thị trạng thái và nút Revoke.
  * [ ] **2.4.4:** Kế thừa purchase order form view hiển thị badge `pdp_status` và trường `ai_agent_id`.
* **Tiêu chuẩn nghiệm thu:**
  * Cài đặt addon vào Odoo 17 (`odoo-bin -u pdp_authorizer -d pdp_db`) thành công không lỗi cú pháp.

---

## PHA 3: TÍCH HỢP TOÀN TRÌNH E2E & ĐO ĐẠC HỌC THUẬT

### Task 3.1: Docker Compose Orchestration Môi Trường Thật
* **File mục tiêu:** `docker-compose.yml`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **3.1.1:** Ghép nối `pdp-server` (Go :50051), `postgres` (:5432), và `odoo17` (:8069).
  * [ ] **3.1.2:** Thiết lập mạng chung `pdp-network` giữa container Odoo và container Go PDP.
  * [ ] **3.1.3:** Kiểm tra lệnh ping gRPC từ bên trong Odoo container sang Go PDP.
* **Tiêu chuẩn nghiệm thu:**
  * `docker compose ps` báo tất cả services đều ở trạng thái `healthy`.

---

### Task 3.2: Tự Động Hóa Kiểm Thử 7 Test Vectors (S1 $\to$ S7)
* **File mục tiêu:** `scripts/run_e2e_scenarios.py`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **3.2.1:** Viết script Python tự động tạo dữ liệu mẫu trên Odoo qua XML-RPC / JSON-RPC.
  * [ ] **3.2.2:** Kích hoạt 7 kịch bản kiểm thử (S1: Bob ALLOW, S2: Bob > $5k, S3: Alice SoD, S4: AI $800, S5: AI $50k, S6: Prompt Injection $10M, S7: Bob mượn AI).
  * [ ] **3.2.3:** Thu thập kết quả phản hồi của từng kịch bản và so sánh với `EVALUATION_MATRIX.md`.
* **Tiêu chuẩn nghiệm thu:**
  * 7/7 kịch bản đạt kết quả mong muốn 100% (100% PASS).

---

### Task 3.3: Thu Thập Số Liệu Thực Nghiệm & Ảnh Chụp Minh Chứng
* **File mục tiêu:** Thư mục `docs/thesis-assets/`
* **Việc cần làm chi tiết (Micro-tasks):**
  * [ ] **3.3.1:** Đo đạc độ trễ toàn trình E2E (Network gRPC + In-Memory Check) và xuất đồ thị phân phối.
  * [ ] **3.3.2:** Chụp ảnh màn hình giao diện Odoo 17 tại thời điểm:
    * Bấm nút Revoke phiên ủy quyền AI.
    * Đơn hàng PO chuyển sang trạng thái `'to approve'` kèm nhắc việc.
    * Đơn hàng vi phạm SoD bị chặn kèm thông báo lỗi `AccessError`.
  * [ ] **3.3.3:** Nhúng ảnh và bảng số liệu vào Chương 4 và Chương 5 của quyển Thuyết minh Đồ án.
* **Tiêu chuẩn nghiệm thu:**
  * Đầy đủ hình vẽ minh chứng thực tế đưa vào báo cáo nghiệm thu trước Hội đồng Khoa học.
