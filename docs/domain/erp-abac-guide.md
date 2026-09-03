# Hướng Dẫn Nghiên Cứu & Tích Hợp ABAC Cho Hệ Thống ERP

Tài liệu này cung cấp hướng dẫn chuyên sâu về việc thiết kế, mô hình hóa và tích hợp **Policy Decision Point (PDP)** hiệu năng cao phục vụ phân quyền chi tiết (**ABAC/PBAC**) trong hệ thống Hoạch định Nguồn lực Doanh nghiệp (**Enterprise Resource Planning - ERP**).

---

## 1. Tại Sao ERP Cần ABAC Thay Vì RBAC Truyền Thống?

Trong các hệ thống ERP truyền thống, việc phân quyền thường dựa trên mô hình **RBAC (Role-Based Access Control)** cứng nhắc, dẫn đến các vấn đề nghiêm trọng:

1. **Bùng nổ vai trò (Role Explosion):**
   - Khi doanh nghiệp mở rộng với nhiều chi nhánh, phòng ban và hạn mức tài chính khác nhau, số lượng vai trò tăng theo cấp số nhân: `Kế toán_Chi nhánh HN`, `Kế toán_Chi nhánh HCM`, `Trưởng phòng_Duyệt 20M`, `Trưởng phòng_Duyệt 50M`, `Giám đốc_Khối IT`...
2. **Logic kiểm tra bị phân mảnh và phân tán (Authorization Leakage):**
   - Các điều kiện so khớp động (ví dụ: `PO.department == User.department`, `PO.amount <= User.limit`, `User.id != Voucher.creator_id`) bị viết cứng rải rác trong tầng Service hoặc câu lệnh `WHERE` của SQL.
   - Khi chính sách công ty thay đổi, lập trình viên phải sửa code và redeploy toàn bộ hệ thống ERP.
3. **Thiếu khả năng kiểm toán tập trung (Centralized Audit Trail):**
   - Không có nơi tập trung để tra cứu xem ai đã quyết định cho phép/cấm một hành động và lý do theo luật nào.

**Giải pháp với Standalone PDP (ABAC/PBAC):**
- Tách toàn bộ logic phân quyền ra khỏi mã nguồn ERP thành một dịch vụ chuyên trách.
- Quản lý chính sách dưới dạng mã khai báo (**Policy-as-Code**) có thể cập nhật nóng (< 300ms) mà không cần downtime hay sửa code.
- Đánh giá trong RAM với độ trễ $< 0.35$ ms và ghi log kiểm toán bất biến tự động.

---

## 2. Ma Trận Thuộc Tính Phân Quyền Trong ERP (Attribute Matrix)

Một yêu cầu phân quyền trong ERP gửi đến PDP thông qua hàm gRPC `CheckAccess(tenant_id, subject, action, resource, context)` mang các thuộc tính:

| Nhóm thuộc tính | Tên thuộc tính | Ý nghĩa nghiệp vụ | Ví dụ |
|---|---|---|---|
| **Chủ thể (Subject)** | `principal.id` | Mã định danh người dùng / hệ thống | `user:alice`, `service:scheduler` |
| | `principal.role` | Vai trò / Chức danh trong tổ chức | `role:department_manager`, `role:cfo` |
| | `principal.department` | Phòng ban / Trung tâm chi phí (Cost Center) | `IT`, `FINANCE`, `SALES`, `HR` |
| | `principal.branch_id` | Chi nhánh / Pháp nhân trực thuộc | `BRANCH_HN`, `BRANCH_HCM`, `LEGAL_VN` |
| | `principal.approval_limit` | Hạn mức tài chính được phép phê duyệt (VNĐ) | `50000000` (50 triệu), `500000000` (500 triệu) |
| | `principal.is_group_auditor` | Cờ kiểm toán viên cấp tập đoàn | `true`, `false` |
| **Tài nguyên (Resource)** | `resource.id` | Mã định danh đối tượng tài nguyên | `resource:purchase_order`, `resource:invoice` |
| | `resource.department` | Phòng ban tạo hoặc sở hữu chứng từ | `IT`, `HR` |
| | `resource.branch_id` | Chi nhánh phát sinh chứng từ | `BRANCH_HN`, `BRANCH_HCM` |
| | `resource.amount` | Giá trị tiền tệ của chứng từ | `25000000`, `1200000000` |
| | `resource.creator_id` | Người khởi tạo chứng từ ban đầu | `user:emma`, `user:bob` |
| | `resource.status` | Trạng thái vòng đời chứng từ | `DRAFT`, `PENDING_APPROVAL`, `APPROVED` |
| **Môi trường (Context)** | `context.ip_address` | Địa chỉ IP của client gửi yêu cầu | `10.1.2.3` (Mạng nội bộ), `192.168.1.5` |
| | `context.request_time` | Thời gian gửi yêu cầu | `08:30:00Z`, `23:15:00Z` |
| | `context.device_status` | Trạng thái bảo mật của thiết bị đầu cuối | `secure`, `compromised` |
| | `context.network_type` | Loại mạng kết nối | `vpn_internal`, `public_wifi` |

---

## 3. Các Mẫu Chính Sách (Policy Patterns) Chuẩn Hóa Cho ERP

### 3.1. Phê Duyệt Đơn Mua Hàng (PO Approval) Theo Hạn Mức & Phòng Ban
```cedar
// Trưởng phòng duyệt PO thuộc phòng ban mình nếu số tiền <= hạn mức và trong giờ hành chính
permit(
    principal in role:department_manager,
    action    == action:APPROVE_PO,
    resource  == resource:purchase_order
)
when {
    principal.department == resource.department &&
    resource.amount <= principal.approval_limit &&
    context.ip_address in "10.0.0.0/8" &&
    context.request_time >= "08:00:00Z" &&
    context.request_time <= "18:00:00Z"
};
```

### 3.2. Phân Tách Nhiệm Vụ (Separation of Duties - SoD) Cho Chứng Từ Tài Chính
*Nguyên tắc kế toán:* Người lập phiếu chi không bao giờ được phép tự duyệt phiếu chi đó.
```cedar
// Cho phép Kế toán trưởng duyệt phiếu chi
permit(
    principal in role:chief_accountant,
    action    == action:APPROVE_PAYMENT,
    resource  == resource:payment_voucher
);

// Luật cấm SoD: Bất kỳ ai là người tạo (creator_id) thì bị cấm duyệt tuyệt đối
forbid(
    principal == any,
    action    == action:APPROVE_PAYMENT,
    resource  == resource:payment_voucher
)
when {
    principal.id == resource.creator_id
};
```

### 3.3. Cách Ly Dữ Liệu Chi Nhánh & Quyền Kiểm Toán Tập Đoàn
```cedar
permit(
    principal in role:accountant,
    action    == action:VIEW_INVOICE,
    resource  == resource:invoice
)
when {
    principal.branch_id == resource.branch_id ||
    principal.is_group_auditor == "true"
};
```

### 3.4. Bảo Mật Dữ Liệu Lương & Ngăn Chặn Truy Cập Ngoài Mạng An Toàn
```cedar
permit(
    principal in role:hr_specialist,
    action    == action:VIEW_PAYROLL,
    resource  == resource:payroll
)
when {
    context.ip_address in "10.10.0.0/16" &&
    context.request_time >= "08:00:00Z" &&
    context.request_time <= "18:00:00Z"
};

// Cấm tuyệt đối khi kết nối từ Wifi công cộng hoặc thiết bị nhiễm mã độc
forbid(
    principal == any,
    action    == any,
    resource  == resource:payroll
)
when {
    context.network_type == "public_wifi" ||
    context.device_status == "compromised"
};
```

### 3.5. Chốt Chặn Tiền Định Cho Tác Tử AI (AI Agent Guardrails) & Quy Chuẩn Định Dạng Số

Trong kỷ nguyên ERP tự hành, các Tác tử AI (AI Copilots/Agents) thực thi lệnh gọi công cụ (Tool-Calls). PDP đóng vai trò là rào chắn tiền định (Deterministic Guardrail) ngăn ngừa tấn công Prompt Injection và ảo giác tài chính:

```cedar
// 1. Cho phép AI Agent tự động duyệt PO nhỏ trong hạn mức ủy quyền
permit(
    principal == agent:procurement_copilot,
    action    == action:APPROVE_PO,
    resource  == resource:purchase_order
)
when {
    context.tool_context == "tool:auto_create_po" &&
    context.amount <= 2000
};

// 2. Rào chắn cấm tuyệt đối AI tự ý duyệt đơn lớn vượt trần tự hành
forbid(
    principal == agent:procurement_copilot,
    action    == action:APPROVE_PO,
    resource  == resource:purchase_order
)
when {
    context.amount > 2000
};

// 3. Rào chắn SoD tổng quát: Cấm duyệt nếu người tạo nằm trong chuỗi ủy quyền
forbid(
    principal == any,
    action    == action:APPROVE_PO,
    resource  == any
)
when {
    context.delegation_chain contains resource.creator_id
};
```

> ⚠️ **BẤT BIẾN KỸ THUẬT QUAN TRỌNG VỀ SERIALIZE SỐ NGUYÊN & CHUỖI ỦY QUYỀN:**
> 1. **Số nguyên thuần túy:** Lõi `evaluator.go` parse số bằng `strconv.ParseInt(val, 10, 64)`. Tuyệt đối không gửi chuỗi tiền tệ định dạng giao diện có dấu chấm/phẩy (`"50.000.000"` VNĐ). Lỗi parse sẽ làm phép so sánh `amount > 2000` trả về `false`, dẫn tới **luật cấm `forbid` bị bypass**! Tầng PEP bắt buộc phải serialize số nguyên thô: `"50000000"`, `"2000"`.
> 2. **Toán tử `contains` cho SoD:** Trong lõi `evaluator.go`, toán tử `in` chỉ áp dụng cho CIDR IP và Role DAG. Để kiểm tra phần tử trong chuỗi ủy quyền, cú pháp bắt buộc là `context.delegation_chain contains resource.creator_id`, với `delegation_chain` được PEP định dạng thành chuỗi ngăn cách bởi dấu phẩy: `"user:alice,agent:copilot"`.
> 3. **Lộ trình Obligation 2 pha:** Trong Pha 1 (hiện tại), `Obligations` (`REQUIRE_HUMAN_APPROVAL`) được tầng Decision Synthesizer ánh xạ dựa trên `matched_policy_id`. Trong Pha 2 (mở rộng học thuật), ngữ pháp DSL sẽ được nâng cấp để hỗ trợ trực tiếp khối `advice { ... }`.
> 4. **Kiến trúc phân tầng bảo vệ 27ns:** Việc xác thực chữ ký số `delegation_proof` (HMAC-SHA256) và kiểm tra In-Memory Revocation Blacklist $O(1)$ được thực thi tại **gRPC Security Interceptor** (tầng Gateway) để bảo vệ ranh giới niềm tin (Trust Boundary), giữ cho lõi đánh giá In-Memory Evaluator đạt vận tốc 27ns và Zero-Alloc.
> 5. **Cơ chế phân giải thuộc tính (Scope Resolution):** Lõi `evaluator.go:131-137` tự động tìm key có tiền tố `resource.<field>` trước, sau đó mới fallback sang `<field>`. Odoo PEP trích xuất dữ liệu từ ORM và bơm sẵn vào request context (`resource.creator_id: "user:alice"`), PDP không bao giờ truy vấn ngược lại CSDL ERP trên đường truyền nóng.

---

## 4. Quy Trình Tích Hợp Vào Kiến Trúc ERP

```text
[ERP Web/Mobile App]
       │
       ▼ (1) HTTP API Request (kèm JWT Token)
[API Gateway / Envoy PEP]
       │
       │ (2) Trích xuất Claims từ JWT & Request Payload
       │ (3) Gửi gRPC: CheckAccess(tenant_id, subject, action, resource, context)
       ▼
[Standalone PDP (Port 50051)]
       │
       ├─ (4) Tra cứu In-Memory Trie: Tenant → Subject → Resource → Action [O(log N)]
       ├─ (5) Kiểm tra phân cấp Role DAG [O(1)]
       ├─ (6) Đánh giá AST Condition (Đoản mạch logic)
       ├─ (7) Ghi log WORM bất đồng bộ qua Ring Buffer xuống PostgreSQL
       │
       ▼ (8) Phản hồi: ALLOW / DENY (+ matched_policy_id)
[API Gateway / Envoy PEP]
       │
       ▼ (9) Cho phép chuyển tiếp tới Microservice Backend HOẶC trả về 403 Forbidden
[ERP Backend Services: Mua Hàng / Kế Toán / Kho / Nhân Sự]
```

---

## 5. Giải Thích Quyết Định & Tuân Thủ Pháp Lý (Compliance & Explainability)

Khi tích hợp với các tiêu chuẩn kiểm toán doanh nghiệp (như SOX, ISO 27001, GDPR), PDP cung cấp endpoint `ExplainDecision`:

**Yêu cầu:**
```json
{
  "tenant_id": "erp-corp-01",
  "subject": "user:alice",
  "action": "APPROVE_PO",
  "resource": "resource:purchase_order",
  "context": {
    "principal.department": "IT",
    "resource.department": "IT",
    "resource.amount": "25000000",
    "context.device_status": "compromised"
  }
}
```

**Phản hồi giải thích từ PDP:**
```json
{
  "decision": "DENY",
  "final_reason": "Yêu cầu bị từ chối bởi luật cấm tường minh: P-PO-05",
  "matched": [
    {
      "policy_id": "P-PO-05",
      "effect": "forbid",
      "policy_text": "forbid(principal == any, action == action:APPROVE_PO, resource == any) when { context.device_status == 'compromised' };"
    }
  ]
}
```

---

## 6. Vận Hành & Quản Trị Bằng Công Cụ `pectl`

Quản trị viên ERP có thể sử dụng công cụ CLI `pectl` để quản lý toàn bộ vòng đời chính sách:

```bash
# 1. Khởi tạo chính sách mới ở trạng thái DRAFT
pectl policy create erp-corp-01 --effect permit --file po_approval.cedar

# 2. Chạy thử giả lập (Simulate) kiểm tra kết quả trước khi phát hành
pectl simulate erp-corp-01 \
  --subject user:bob \
  --action APPROVE_PO \
  --resource resource:purchase_order \
  --context-file test_context.json \
  --draft-file po_approval.cedar

# 3. Kích hoạt và nạp nóng chính sách lên RAM Engine toàn cluster
pectl policy publish erp-corp-01 pol_8f912c3b

# 4. Kiểm tra sức khỏe và chỉ số PDP
pectl health
pectl metrics
```
