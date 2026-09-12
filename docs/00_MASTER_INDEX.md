# Standalone Policy Engine — Master Documentation Index

> 🎯 **Dự án**: Standalone In-Memory Go Policy Decision Point (PDP) & Odoo 17 ERP Delegation Authorizer  
> 🎓 **Đề tài Đồ án Tốt nghiệp PTIT**: *"Xây dựng cơ chế Policy Decision Point hỗ trợ ủy quyền có kiểm soát (Delegation-Aware Authorization) cho Tác tử AI trong hệ thống ERP — Nghiên cứu triển khai và đánh giá thực nghiệm trên nền tảng Odoo"*  
> **Trạng thái hiện tại**: **CORE ENGINE VERIFIED; FULL SYSTEM NOT PRODUCTION READY**
>
> **Tài liệu có thẩm quyền về hiện trạng**: [`CURRENT_STATE_AUDIT.md`](./technical-spec/CURRENT_STATE_AUDIT.md)
>
> **Liên kết đề cương - PDP - Odoo Project 2**: [`THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`](./thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md)
>
> Các roadmap/benchmark cũ mô tả mục tiêu hoặc kết quả lịch sử và không được dùng để tuyên bố production readiness nếu mâu thuẫn với current-state audit.

---

## 🗺️ Bản Đồ Cấu Trúc Tài Liệu Chuẩn Hóa (Documentation Taxonomy)

Toàn bộ hệ thống tài liệu đã được tinh gọn, loại bỏ 100% các tài liệu rác cũ hoặc mâu thuẫn kiến trúc, quy tụ về cấu trúc phân cấp chuẩn hóa:

```text
docs/
├── 00_MASTER_INDEX.md                 # Mục lục tổng thể & Điều hướng hệ thống (Tài liệu này)
│
├── 🎓 thesis-proposal/                 # Đề Cương & Khung Nghiên Cứu Đồ Án Tốt Nghiệp PTIT
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md    # Bản thuyết minh đề cương 4 RQs chuẩn khoa học
│   ├── THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md                  # Đối chiếu đề cương, PDP hiện tại và Odoo Project 2
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.docx  # Bản Word lịch sử của đề cương
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC_2026_09_11.docx # Bản Word có phụ lục hiệu đính hiện trạng
│   └── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.pdf   # Bản PDF lịch sử; dùng bản Word 2026-09-11 hoặc Markdown hiệu đính làm nguồn hiện hành
│
├── 📑 technical-spec/                 # Đặc tả kỹ thuật, audit hiện trạng và release gates
│   ├── CURRENT_STATE_AUDIT.md         # Nguồn sự thật về implementation tại commit được audit
│   ├── PRODUCTION_READINESS_CHECKLIST.md # Release gates bắt buộc trước production
│   ├── SKILL_CATALOG_AUDIT.md         # Audit skill hiện có và thiết kế catalog remediation
│   ├── REMEDIATION_EXECUTION_PLAN.md  # Thứ tự khắc phục P0/P1 và tiêu chí hoàn thành
│   ├── IMPLEMENTATION_TASK_BOARD.md   # Backlog task nhỏ, trạng thái và bằng chứng
│   ├── DELEGATION_REPLAY_IDEMPOTENCY.md # Semantics retry, nonce ledger và key-ring rotation
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

## 🧭 Chi Tiết Điều Hướng Tài Liệu Kỹ Thuật (`docs/technical-spec/`)

| # | Tài liệu | Nội dung cốt lõi | Đối tượng phục vụ |
|---|---|---|---|
| **0A** | [`CURRENT_STATE_AUDIT.md`](./technical-spec/CURRENT_STATE_AUDIT.md) | Audit implementation, evidence register, P0/P1 findings và phạm vi các tuyên bố đã xác minh. Đây là tài liệu ưu tiên cao nhất khi đánh giá trạng thái hiện tại. | Maintainer, Security Auditor, Hội đồng Khoa học |
| **0B** | [`PRODUCTION_READINESS_CHECKLIST.md`](./technical-spec/PRODUCTION_READINESS_CHECKLIST.md) | Release gates về identity, delegation, consistency, revocation, obligation, audit, protocol, deployment và E2E. | Engineering Lead, QA, DevOps |
| **0C** | [`SKILL_CATALOG_AUDIT.md`](./technical-spec/SKILL_CATALOG_AUDIT.md) | Quyết định giữ/sửa/tách/thêm skill, scope của catalog mục tiêu và thứ tự remediation. | Maintainer, AI/Agent Designer |
| **0D** | [`REMEDIATION_EXECUTION_PLAN.md`](./technical-spec/REMEDIATION_EXECUTION_PLAN.md) | Kế hoạch khắc phục P0/P1 theo dependency, test bắt buộc, điều kiện hoàn thành và các quyết định cần chủ dự án chốt. | Engineering Lead, Security, QA, DevOps |
| **0E** | [`IMPLEMENTATION_TASK_BOARD.md`](./technical-spec/IMPLEMENTATION_TASK_BOARD.md) | Task board nguyên tử theo từng wave, trạng thái thật và bằng chứng/blocker của từng phần sửa. | Maintainer, Engineering Lead, QA |
| **0F** | [`DELEGATION_REPLAY_IDEMPOTENCY.md`](./technical-spec/DELEGATION_REPLAY_IDEMPOTENCY.md) | Contract idempotent `CheckAccess`, nonce ledger tại Odoo transaction và quy trình xoay key-ring. | Security, Odoo PEP, Maintainer |
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
| **12** | [`IMPLEMENTATION_ROADMAP.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/IMPLEMENTATION_ROADMAP.md) | Roadmap lịch sử; chỉ dùng cùng current-state audit và task board, không phải bằng chứng rằng Odoo addon hoặc E2E đã hoàn thành. | Quản lý dự án, Nghiệm thu mã nguồn |

---

## 📊 Tóm Tắt Bằng Chứng Đã Xác Minh

* **Go core benchmark**: `BenchmarkEvaluatorLatency` đạt 320.2–341.8 ns/op, `0 B/op`, `0 allocs/op` trong audit ngày 2026-09-11. Phạm vi này không bao gồm JWT, HMAC, GC tracking, metrics, audit, gRPC hoặc network.
* **Unit and in-process test suite**: `go test -count=1 ./...` PASS; `go vet ./...` PASS.
* **Delegation vectors**: 7/7 test cases trong `tests/e2e_delegation_test.go` PASS ở chế độ in-process. Chúng chưa phải bằng chứng Odoo/container/network E2E.
* **Production readiness**: BLOCKED bởi các finding P0 trong [`CURRENT_STATE_AUDIT.md`](./technical-spec/CURRENT_STATE_AUDIT.md).

Các vector logic đã chạy:
  * ✅ `TC-01`: Manager tự tạo PO và tự duyệt $\to$ **DENY (SoD Collision)**.
  * ✅ `TC-02`: AI Agent duyệt hộ PO do Manager tạo $\to$ **DENY (SoD Delegation Chain)**.
  * ✅ `TC-03`: AI Agent tự động duyệt PO vendor hợp lệ $\le \$2,000$ $\to$ **ALLOW**.
  * ✅ `TC-04`: AI Agent duyệt PO vượt trần tự hành $>\$2,000$ $\to$ **DENY (Guardrail Limit)**.
  * ✅ `TC-05`: Kẻ tấn công sửa context số tiền $\to$ **403 PermissionDenied (HMAC Tampered)**.
  * ✅ `TC-06`: Test gọi `RevokeDelegation` trên cùng server in-process rồi kiểm tra request tiếp theo $\to$ **DENY trong process đó**.
  * ✅ `TC-07`: AI Agent mang token đã quá hạn TTL $\to$ **403 PermissionDenied (Expired Token)**.
