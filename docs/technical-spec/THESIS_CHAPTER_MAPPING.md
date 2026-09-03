# THESIS_CHAPTER_MAPPING.md — Ánh Xạ Luận Văn Thuyết Minh Đồ Án (PTIT Capstone Defense)

Tài liệu này đóng vai trò là **kim chỉ nam dẫn đường (Master Mapping)** liên kết toàn bộ mã nguồn, cấu trúc dữ liệu, và kết quả thực nghiệm trong repository vào **5 Chương của Quyển Thuyết minh Đồ án Tốt nghiệp (80–100 trang)**.

---

## 1. Ma Trận Giải Quyết Câu Hỏi Nghiên Cứu (Research Questions Matrix)

| Câu hỏi nghiên cứu | Trọng tâm học thuật | Chương giải quyết | Mô hình toán & Công thức | File mã nguồn minh chứng |
|---|---|---|---|---|
| **RQ1 (Unified Subject & Delegation)** | Biểu diễn định danh Người, AI Agent, Chuỗi ủy quyền 1-Hop | **Chương 2 & Chương 3** | $\Delta = \langle \mathcal{U}_{\text{root}}, \mathcal{A}_{\text{exec}}, \Sigma_{\text{scope}}, \Omega_{\text{constraints}}, \mathcal{C}_{\text{chain}} \rangle$<br>Input 4-Tuple $\mathcal{I} = \langle \mathcal{S}, \mathcal{A}, \mathcal{R}, \mathcal{C} \rangle$ | [`proto/v1/policy.proto`](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto)<br>[`internal/engine/evaluator.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go) |
| **RQ2 (Machine-Speed In-Memory Runtime)** | Độ trễ nano-giây (27ns), 0 byte heap alloc, thông lượng > 30M RPS | **Chương 3 & Chương 5** | Trie Hashing $O(\log N)$ + DAG Closure $O(1)$ + Copy-On-Write $\mathcal{O}_{\text{COW}}$ | [`internal/engine/trie.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go)<br>[`internal/engine/dag.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go)<br>[`tests/benchmark_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/benchmark_test.go) |
| **RQ3 (Deterministic Guardrails & Safety)** | Rào chắn 3 trạng thái (`ALLOW`/`DENY`/`REQUIRE_APPROVAL`), chặn Prompt Injection | **Chương 2 & Chương 4** | $\mathcal{P}_{\text{effective}}(\mathcal{A} \mid \mathcal{U}, t) = \mathcal{P}_{\text{active}}(\mathcal{U}, t) \cap \mathcal{S}_{\text{delegation}} \cap \Omega_{\text{guardrails}}$<br>Generalized SoD: $\mathcal{U}_{\text{creator}} \notin \mathcal{C}_{\text{chain}}$ | [`internal/engine/decision.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/decision.go)<br>[`tests/ai_agent_guardrails_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/ai_agent_guardrails_test.go) |
| **RQ4 (Empirical 3-Tier ERP Evaluation)** | Đánh giá thực nghiệm 3 chiều trên Odoo ERP so với OPA và SQL ir.rule | **Chương 4 & Chương 5** | Triệt tiêu TOCTOU qua `RevocationMap` $O(1)$<br>Non-Rollback PEP State Machine | [`custom_addons/pdp_authorizer/`](file:///E:/Projects/ERP_Mastery_Hub/02_Project_2_Odoo_Go_PDP_Approval/custom_addons/pdp_authorizer/)<br>[`docs/technical-spec/EVALUATION_MATRIX.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/EVALUATION_MATRIX.md) |

---

## 2. Bản Đồ Nội Dung Chi Tiết Theo 5 Chương Thuyết Minh

```
┌─────────────────────────────────────────────────────────────────────────────┐
│              CẤU TRÚC 5 CHƯƠNG THUYẾT MINH ĐỒ ÁN TỐT NGHIỆP PTIT            │
├─────────────────────────────────────────────────────────────────────────────┤
│ CHƯƠNG 1: GIỚI THIỆU TỔNG QUAN, BỐI CẢNH VÀ ĐẶT VẤN ĐỀ                      │
│ • Bối cảnh Kỷ nguyên Doanh nghiệp Tự hành (Autonomous Enterprise)            │
│ • Khủng hoảng phân quyền tĩnh: Role Explosion, Rò rỉ SQL, Vi phạm SoD       │
│ • Rủi ro an toàn AI: Ảo giác, Prompt Injection, Lỗ hổng OWASP LLM06         │
│ • Mục tiêu nghiên cứu, phạm vi 1-Hop trên Odoo 17 và 4 câu hỏi RQ1–RQ4       │
├─────────────────────────────────────────────────────────────────────────────┤
│ CHƯƠNG 2: CƠ SỞ LÝ THUYẾT VÀ HÌNH THỨC HÓA MÔ HÌNH ỦY QUYỀN                 │
│ • Chuẩn NIST SP 800-162 ABAC (PEP - PDP - PIP - PAP) & Zero Trust SP 800-207│
│ • Không gian thuộc tính Input 4-Tuple I = <S, A, R, C> và Hàm quyết định D   │
│ • Hình thức hóa bộ ngũ ủy quyền có kiểm soát: Delta = <U, A, Sigma, Omega, C>│
│ • 3 Bất biến an toàn: Monotonic Attenuation, Generalized SoD, Proof HMAC     │
├─────────────────────────────────────────────────────────────────────────────┤
│ CHƯƠNG 3: THIẾT KẾ KIẾN TRÚC VÀ LÕI ĐỘNG CƠ IN-MEMORY PDP                   │
│ • Kiến trúc 2 tầng: Layer 1 (Security Interceptor) vs Layer 2 (Hot-Path Core)│
│ • Cấu trúc chỉ mục Radix Trie FNV-1a uint64 (Trie Index O(log N))           │
│ • Đồ thị Role DAG & Thuật toán tiền tính toán Bao đóng bắc cầu O(1) query    │
│ • Trình biên dịch Pratt Parser, Lexer, Type Checker & Capped AST Depth <= 15 │
│ • Lộ trình 2 pha cho Obligations: Metadata Mapping -> EBNF Advice Block      │
│ • Đồng bộ Stateless: Postgres Monotonic Sequence LISTEN/NOTIFY & WORM Logger │
├─────────────────────────────────────────────────────────────────────────────┤
│ CHƯƠNG 4: HIỆN THỰC HÓA VÀ TÍCH HỢP HỆ THỐNG TRÊN NỀN TẢNG ODOO ERP         │
│ • Xây dựng Custom Addon Odoo 17 (pdp_authorizer): Kế thừa purchase.order     │
│ • Thiết kế PID-Safe gRPC Persistent Client cho Odoo Multi-Process Workers    │
│ • Cơ chế điều phối Non-Rollback: State 'to approve' kèm Activity nhắc sếp    │
│ • Model CSDL pdp.delegation.grant, sinh mã băm HMAC-SHA256 delegation_proof │
│ • Ngữ pháp DSL P2P và toán tử SoD contains (khớp evaluator.go:387)           │
├─────────────────────────────────────────────────────────────────────────────┤
│ CHƯƠNG 5: ĐÁNH GIÁ THỰC NGHIỆM 3 CHIỀU, KẾT LUẬN VÀ HƯỚNG PHÁT TRIỂN        │
│ • 1. Functional: Kiểm chứng 100% PASS 7 kịch bản ERP P2P biên               │
│ • 2. Security: Phân tích STRIDE, chặn Prompt Injection $10M trong 286.3 ns,  │
│      triệt tiêu TOCTOU trong < 1 µs bằng In-Memory RevocationMap O(1)        │
│ • 3. Performance: Benchmark 36.8M RPS (20 cores), 27.12 ns, 0 allocs/op      │
│ • Bảng so sánh 7 chiều với OPA Sidecar và Odoo ir.rule SQL Native           │
│ • Kết luận đóng góp khoa học và hướng mở rộng kiến trúc Two-Tier SAP S/4HANA │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Danh Mục Hình Vẽ & Biểu Bảng Trích Xuất Vào Thuyết Minh

| Ký hiệu trong Thuyết minh | Tên hình vẽ / Biểu bảng trích dẫn | Nguồn tài liệu kỹ thuật trong Repo |
|---|---|---|
| **Hình 1.1** | Mô hình rủi ro Excessive Agency khi AI gọi Tool ERP | [`docs/technical-spec/THREAT_MODEL.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/THREAT_MODEL.md) |
| **Hình 2.1** | Kiến trúc NIST SP 800-162 mở rộng cho AI Agent | [`docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md) |
| **Hình 3.1** | Sơ đồ khối kiến trúc phân tầng 2 lớp (Layer 1 Interceptor vs Layer 2 Core) | [`docs/technical-spec/ARCH_SPEC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/ARCH_SPEC.md) |
| **Hình 3.2** | Cấu trúc chỉ mục Radix Trie 4 cấp FNV-1a uint64 | [`internal/engine/trie.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go) |
| **Hình 3.3** | Thuật toán DFS Transitive Closure biến đổi Role DAG sang $O(1)$ | [`internal/engine/dag.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go) |
| **Hình 4.1** | Máy trạng thái Odoo ORM Non-Rollback phối hợp với Go PDP | [`docs/technical-spec/PEP_ODOO_INTEGRATION.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/PEP_ODOO_INTEGRATION.md) |
| **Hình 4.2** | Sơ đồ tương tác triệt tiêu TOCTOU bằng `RevocationMap` $O(1)$ | [`docs/technical-spec/SECURITY_INVARIANTS.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/SECURITY_INVARIANTS.md) |
| **Hình 5.1** | Biểu đồ thông lượng mở rộng tuyến tính từ 1 Core đến 20 Cores | [`docs/technical-spec/BENCHMARK_REPRODUCIBILITY.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/BENCHMARK_REPRODUCIBILITY.md) |
| **Bảng 3.1** | Ma trận trách nhiệm các thành phần hệ thống (Component Matrix) | [`docs/technical-spec/ARCH_SPEC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/ARCH_SPEC.md) |
| **Bảng 4.1** | Bảng quy ước Scope Resolution và Fallback Rules trong Go Engine | [`docs/technical-spec/PROTOCOL_CONTRACT.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/PROTOCOL_CONTRACT.md) |
| **Bảng 5.1** | Ma trận kết quả kiểm thử 7 kịch bản biên Enterprise P2P | [`docs/technical-spec/EVALUATION_MATRIX.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/EVALUATION_MATRIX.md) |
| **Bảng 5.2** | Bảng so sánh hiệu năng 7 chiều: Go PDP vs OPA vs Odoo `ir.rule` | [`docs/technical-spec/EVALUATION_MATRIX.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/EVALUATION_MATRIX.md) |
