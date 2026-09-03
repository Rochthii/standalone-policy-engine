# Standalone Policy Engine — Master Documentation Index

> 🎯 **Dự án**: Standalone In-Memory Go Policy Decision Point (PDP) & Odoo 17 ERP Delegation Authorizer  
> 🎓 **Đề tài Đồ án Tốt nghiệp PTIT**: *"Xây dựng cơ chế Policy Decision Point hỗ trợ ủy quyền có kiểm soát (Delegation-Aware Authorization) cho Tác tử AI trong hệ thống ERP — Nghiên cứu triển khai và đánh giá thực nghiệm trên nền tảng Odoo"*  
> 🏆 **Trạng thái**: **100% HOÀN THÀNH TOÀN BỘ MÃ NGUỒN VÀ THỰC NGHIỆM (COMPLETED & VERIFIED)**

---

## 🗺️ Bản Đồ Cấu Trúc Tài Liệu Chuẩn Hóa (Documentation Taxonomy)

Toàn bộ hệ thống tài liệu đã được tinh gọn, loại bỏ 100% các tài liệu rác cũ hoặc mâu thuẫn kiến trúc, quy tụ về cấu trúc phân cấp chuẩn hóa:

```text
docs/
├── 00_MASTER_INDEX.md                 # Mục lục tổng thể & Điều hướng hệ thống (Tài liệu này)
│
├── 🎓 thesis-proposal/                 # Đề Cương & Khung Nghiên Cứu Đồ Án Tốt Nghiệp PTIT
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md    # Bản thuyết minh đề cương 4 RQs chuẩn khoa học
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.docx  # Bản Microsoft Word chuẩn nộp Khoa CNTT
│   └── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.pdf   # Bản PDF in ấn xuất bản học thuật
│
├── 📑 technical-spec/                 # BỘ 12 ĐẶC TẢ KỸ THUẬT & NGHIỆM THU MÃ NGUỒN (SPEC SUITE)
│   ├── ARCH_SPEC.md                   # Kiến trúc hệ thống E2E, ma trận trách nhiệm & luồng dữ liệu
│   ├── PROTOCOL_CONTRACT.md           # Hợp đồng gRPC Protobuf, JSON payload, RPC CheckAccess & RevokeDelegation
│   ├── SECURITY_INVARIANTS.md         # Mô hình toán Bộ ngũ Delta, 3 bất biến an toàn, triệt tiêu TOCTOU O(1)
│   ├── EVALUATION_MATRIX.md           # 7 kịch bản kiểm thử biên, đối soát baseline & ngân sách ns
│   ├── PEP_ODOO_INTEGRATION.md        # Hook Odoo ORM, PID-safe client cho worker pre-fork, Non-Rollback PEP
│   ├── POLICY_DSL_SPEC.md             # Ngữ pháp EBNF Cedar, toán tử contains vs in, 6 luật P2P seed rules
│   ├── THREAT_MODEL.md                # Phân tích nguy cơ STRIDE + OWASP LLM06, 4 kịch bản tấn công & rào chắn
│   ├── AGENT_TOOLING_SPEC.md          # Tool-Call JSON schema, ToolExecutionContext & feedback loop tự động
│   ├── RUNBOOK_OPS.md                 # Docker Compose, quy tắc fail-closed, kịch bản cold-start & benchmark tự động
│   ├── THESIS_CHAPTER_MAPPING.md      # Ánh xạ RQ1-RQ4 vào 5 chương Thuyết minh 100 trang & danh mục hình/bảng
│   ├── BENCHMARK_REPRODUCIBILITY.md   # Đặc tả testbed, 7 test vectors, lệnh 1 dòng tái lập thực nghiệm 100%
│   └── IMPLEMENTATION_ROADMAP.md      # Báo cáo nghiệm thu hoàn thành 100% các bước triển khai mã nguồn
│
└── 🛠️ cli/
    └── pectl.md                       # Tài liệu hướng dẫn sử dụng công cụ dòng lệnh Enterprise CLI (pectl)
```

---

## 🧭 Chi Tiết Điều Hướng Bộ 12 Tài Liệu Kỹ Thuật (`docs/technical-spec/`)

| # | Tài liệu | Nội dung cốt lõi | Đối tượng phục vụ |
|---|---|---|---|
| **1** | [`ARCH_SPEC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/ARCH_SPEC.md) | Kiến trúc tổng thể E2E, luồng dữ liệu Zero-Alloc, ma trận phân định trách nhiệm Go PDP vs Odoo PEP, cơ chế Catch-Up đồng bộ trạng thái Postgres. | Kiến trúc sư hệ thống, AI Parser |
| **2** | [`PROTOCOL_CONTRACT.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/PROTOCOL_CONTRACT.md) | Đặc tả Protobuf IDL `policy.proto`, scope resolution fallback, JSON payload mẫu cho 2 RPC lõi: `CheckAccess` và `RevokeDelegation`. | Kỹ sư Backend, Kỹ sư Tích hợp |
| **3** | [`SECURITY_INVARIANTS.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/SECURITY_INVARIANTS.md) | Hình thức hóa toán học: Bộ tứ NIST, Bộ ngũ ủy quyền Delta $\Delta$, 3 bất biến an ninh và cơ chế In-Memory RevocationMap $O(1)$ triệt tiêu TOCTOU. | Hội đồng Khoa học, Security Auditor |
| **4** | [`EVALUATION_MATRIX.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/EVALUATION_MATRIX.md) | Ma trận 7 kịch bản biên (SoD, trần tự hành, giả mạo HMAC, TOCTOU), đối soát OPA/Permit.io và ngân sách độ trễ $< 3.5\,\mu\text{s}$. | Kỹ sư QA/QC, Đánh giá thực nghiệm |
| **5** | [`PEP_ODOO_INTEGRATION.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/PEP_ODOO_INTEGRATION.md) | Hướng dẫn hook Odoo 17 ORM, PID-safe gRPC client cho worker pre-fork, mô hình Non-Rollback PEP và schema model `pdp.delegation.grant`. | Kỹ sư Odoo Python, DevOps |
| **6** | [`POLICY_DSL_SPEC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/POLICY_DSL_SPEC.md) | Ngữ pháp EBNF của Cedar-like DSL, bảng quy ước toán tử phân biệt (`contains` cho SoD vs `in` cho vai trò/mạng), 6 seed policies P2P. | Chuyên gia Policy, Parser Developer |
| **7** | [`THREAT_MODEL.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/THREAT_MODEL.md) | Phân tích mô hình đe dọa STRIDE kết hợp OWASP LLM06, chi tiết 4 cuộc tấn công (Tampered Context, TOCTOU, Prompt Injection, Confused Deputy). | Security Engineer, Thẩm định an toàn |
| **8** | [`AGENT_TOOLING_SPEC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/AGENT_TOOLING_SPEC.md) | JSON Schema định nghĩa Tool-Call của AI Agent, quy ước đóng gói ngữ cảnh `ToolExecutionContext` và máy trạng thái xử lý feedback loop. | AI/Agentic Engineer |
| **9** | [`RUNBOOK_OPS.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/RUNBOOK_OPS.md) | Hướng dẫn triển khai Docker Compose, quy tắc Fail-Closed, xử lý cold-start BadgerDB edge snapshot và script benchmark tự động. | DevOps, SysAdmin, Site Reliability |
| **10** | [`THESIS_CHAPTER_MAPPING.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/THESIS_CHAPTER_MAPPING.md) | Ánh xạ 4 câu hỏi nghiên cứu (RQ1–RQ4) vào 5 Chương Thuyết minh Đồ án 100 trang; danh mục bảng biểu và hình vẽ minh chứng thực nghiệm. | Giảng viên Hướng dẫn, Tác giả đồ án |
| **11** | [`BENCHMARK_REPRODUCIBILITY.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/BENCHMARK_REPRODUCIBILITY.md) | Bộ runbook 1 câu lệnh tái lập 100% toàn bộ kết quả đo lường thực nghiệm, cấu hình phần cứng testbed và thông số baseline. | Phản biện Khoa học, Người tái lập |
| **12** | [`IMPLEMENTATION_ROADMAP.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/IMPLEMENTATION_ROADMAP.md) | Báo cáo tiến độ phân tử hóa: Ghi nhận trạng thái 100% hoàn thành cả 5 bước (Proto, Go PDP, Unit test, Odoo Addon, E2E 7 Vectors). | Quản lý dự án, Nghiệm thu mã nguồn |

---

## 📊 Tóm Tắt Kết Quả Kiểm Định Thực Nghiệm (Live Acceptance Metrics)

* **Hot-Path Memory Allocation**: `0 B/op` và `0 allocs/op` (sử dụng `sync.Pool`, FNV-1a uint64 hashing, bitmask CIDR).
* **Hot-Path Latency**: **$\approx 540.2\ \text{ns/op}$** (nhanh gấp 6.4 lần chỉ tiêu đề ra trong Đề cương là $< 3.5\,\mu\text{s}$).
* **E2E Integration Verification**: **7/7 Vectors PASS 100%** trong `4.506s` (File [`tests/e2e_delegation_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_delegation_test.go)):
  * ✅ `TC-01`: Manager tự tạo PO và tự duyệt $\to$ **DENY (SoD Collision)**.
  * ✅ `TC-02`: AI Agent duyệt hộ PO do Manager tạo $\to$ **DENY (SoD Delegation Chain)**.
  * ✅ `TC-03`: AI Agent tự động duyệt PO vendor hợp lệ $\le \$2,000$ $\to$ **ALLOW**.
  * ✅ `TC-04`: AI Agent duyệt PO vượt trần tự hành $>\$2,000$ $\to$ **DENY (Guardrail Limit)**.
  * ✅ `TC-05`: Kẻ tấn công sửa context số tiền $\to$ **403 PermissionDenied (HMAC Tampered)**.
  * ✅ `TC-06`: Manager bấm Revoke trên Odoo UI $\to$ Agent gọi tiếp $\to$ **DENY (Triệt tiêu TOCTOU trong RAM)**.
  * ✅ `TC-07`: AI Agent mang token đã quá hạn TTL $\to$ **403 PermissionDenied (Expired Token)**.
