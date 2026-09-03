# Standalone Policy Engine — Master Documentation Index

Tài liệu này là sơ đồ mục lục tổng thể điều hướng toàn bộ hệ thống tài liệu kỹ thuật, nghiên cứu học thuật và kiến trúc doanh nghiệp của dự án **Standalone Policy Engine**.

---

## 🗺️ Bản Đồ Cấu Trúc Tài Liệu (Documentation Taxonomy)

```text
docs/
├── 00_MASTER_INDEX.md                 # Mục lục tổng thể (Tài liệu này)
│
├── 🎓 thesis-proposal/                 # Đề Cương & Khung Nghiên Cứu Đồ Án Tốt Nghiệp 2029
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md    # Bản thuyết minh đề cương 4 RQs
│   ├── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.docx  # Bản Microsoft Word chuẩn nộp Khoa
│   └── DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.pdf   # Bản PDF xuất bản chuẩn học thuật
│
├── 📑 technical-spec/                 # BỘ 11 TÀI LIỆU KỸ THUẬT & NGHIỆM THU ĐỒ ÁN (SPEC SUITE)
│   ├── ARCH_SPEC.md                   # Kiến trúc hệ thống, data flow & ma trận trách nhiệm
│   ├── PROTOCOL_CONTRACT.md           # Hợp đồng gRPC Protobuf, scope resolution & JSON payload
│   ├── SECURITY_INVARIANTS.md         # Mô hình toán Bộ ngũ Delta, 3 bất biến & TOCTOU O(1)
│   ├── EVALUATION_MATRIX.md           # 7 kịch bản kiểm thử biên, đối soát baseline & ngân sách ns
│   ├── PEP_ODOO_INTEGRATION.md        # Hook Odoo ORM, PID-safe client, non-rollback state machine
│   ├── POLICY_DSL_SPEC.md             # Ngữ pháp EBNF Cedar, toán tử contains vs in, P2P seed rules
│   ├── THREAT_MODEL.md                # Phân tích nguy cơ STRIDE + OWASP LLM06, 4 kịch bản tấn công
│   ├── AGENT_TOOLING_SPEC.md          # Tool-Call JSON schema, ToolExecutionContext & feedback loop
│   ├── RUNBOOK_OPS.md                 # Docker Compose, fail-closed rule, BadgerDB cold start, script
│   ├── THESIS_CHAPTER_MAPPING.md      # Ánh xạ RQ1-RQ4 vào 5 chương Thuyết minh 100 trang & figures
│   └── BENCHMARK_REPRODUCIBILITY.md   # Đặc tả testbed, 7 test vectors, lệnh 1 dòng tái lập 100%
│
├── 🗺️ career-roadmap/                 # Lộ Trình 4 Giai Đoạn: SE + ERP + AI Vibe Coding
│   ├── 00_OVERVIEW.md                 # Tổng quan 3 trục căn tính First Principles
│   ├── 01_PHASE_1_ODOO_FOUNDATION.md  # Giai đoạn 1: Nắm bắt Odoo & Lập trình phân tán
│   ├── 02_PHASE_2_SAP_ENTERPRISE.md   # Giai đoạn 2: Tiếp cận chuẩn mực SAP & Two-Tier ERP
│   ├── 03_PHASE_3_CAPSTONE_INTERNSHIP.md # Giai đoạn 3: Đồ án tốt nghiệp & Săn thực tập
│   └── 04_PHASE_4_ENTERPRISE_ARCHITECT.md # Giai đoạn 4: Nâng tầm Enterprise Architect
│
├── 📐 requirements/                   # Yêu Cầu Kỹ Thuật Hệ Thống (SRS)
│   └── requirements.md                # Đặc tả 10 Yêu cầu Chức năng (FR) & Phi chức năng (NFR)
│
├── 🏢 domain/                         # Mô Hình Nghiệp Vụ & ERP ABAC
│   ├── domain-model.md                # Đặc tả 10 Thực thể cốt lõi & Sơ đồ ERD
│   └── erp-abac-guide.md              # 4 Kịch bản ERP (PO Limits, SoD, Branch, Payroll)
│
├── 📜 policy-language/                # Ngôn Ngữ Khai Báo Chính Sách (DSL Spec)
│   ├── policy-language-overview.md    # Triết lý thiết kế Policy-as-Code
│   ├── grammar-specification.md       # Đặc tả ngữ pháp EBNF & Lexer/Parser
│   ├── syntax-reference.md            # Hướng dẫn cú pháp toán tử & kiểu dữ liệu
│   ├── semantic-rules.md              # Quy tắc ngữ nghĩa & Thừa kế vai trò
│   ├── evaluation-model.md            # Thuật toán quyết định (Deny-by-Default, Forbid-Overrides)
│   └── examples.md                    # Các ví dụ thực tế về RBAC, ABAC, PBAC
│
├── ⚡ evaluation-engine/              # Động Cơ Đánh Giá In-Memory (Core Engine)
│   ├── ast-design.md                  # Cấu trúc Node AST trong bộ nhớ RAM
│   ├── parser-design.md               # Thiết kế Lexer & Pratt Parser đệ quy
│   ├── compiler-design.go.md          # Tối ưu hóa compile, bitmask IP & Unix Timestamp
│   ├── execution-engine.md            # Thuật toán Radix Trie FNV-1a & Role DAG Closure
│   ├── decision-model.md              # Quyết định 3 trạng thái (ALLOW / DENY / REQUIRE_APPROVAL)
│   └── caching-strategy.md            # Đồng bộ hóa cache động qua Redis Pub/Sub
│
├── 🏛️ architecture/                   # Kiến Trúc Hệ Thống (C4 Architecture)
│   ├── system-overview.md             # Tổng quan luồng PEP-PDP-PIP-PAP
│   ├── logical-architecture.md        # Sơ đồ khối các module logic nội bộ
│   ├── physical-architecture.md       # Quản lý luồng Go-routine & Lock-Free COW
│   ├── deployment-architecture.md     # Topology Cluster & Sidecar với Envoy L7
│   ├── data-flow.md                   # Luồng nạp chính sách và đánh giá quyền
│   └── sequence-diagrams.md           # Sơ đồ tuần tự bắt tay gRPC
│
├── 🔒 security/                       # An Toàn Bảo Mật & AI Guardrails
│   ├── security-overview.md           # Trust boundaries & mTLS X.509
│   ├── policy-tampering.md            # Phòng chống sửa đổi chính sách qua chữ ký ED25519
│   ├── tenant-isolation.md            # Cách ly logic & RAM đa khách thuê (Multi-Tenancy)
│   └── authorization-bypass.md        # Fail-closed design và chống bypass gRPC
│
├── 📊 performance/                    # Hiệu Năng & Ma Trận Kiểm Thử Tải
│   ├── latency-budget.md              # Phân bổ ngân sách độ trễ toàn trình < 0.35ms
│   ├── memory-model.md                # Zero Heap Allocation & sync.Pool
│   └── benchmark-matrix.md            # Ma trận kiểm thử tải từ 10 đến 100k policies (> 1M RPS)
│
├── 💾 data/                           # Lưu Trữ Dữ Liệu & Audit Log
│   └── data-model.md                  # Schema PostgreSQL, BadgerDB & Append-Only Logger
│
├── 🔌 api/                            # Giao Diện Lập Trình (gRPC & REST API)
│   ├── decision-api.md                # gRPC CheckAccess & REST POST decisions
│   ├── management-api.md              # HTTP API quản trị CRUD chính sách
│   └── explain-api.md                 # gRPC ExplainDecision API
│
├── 💻 cli/                            # Công Cụ Dòng Lệnh Quản Trị
│   └── pectl.md                       # Tài liệu hướng dẫn sử dụng công cụ CLI pectl
│
├── 🗺️ roadmap/                         # Kế Hoạch Phát Triển & Sprints
│   └── roadmap.md                     # Lộ trình 9 Phases & Kế hoạch chia Sprint
│
├── 📂 project-structure/              # Cấu Trúc Mã Nguồn Chuẩn
│   └── project-layout.md              # Bố trí thư mục Standard Go Project Layout
│
├── 🧪 testing/                        # Chiến Lược Kiểm Thử 3 Chiều
│   └── testing-strategy.md            # Functional, Security Threat Model, & Performance Benchmark
│
├── 🛠️ operations/                     # Vận Hành & Cứu Hộ Hệ Thống
│   └── runbooks.md                    # Giám sát SLOs & cẩm nang cứu hộ sự cố
│
└── 📝 adr/                            # Quyết Định Kiến Trúc Đã Phê Duyệt (ADR)
    └── adr-001-policy-engine-design.md # Quyết định kiến trúc cốt lõi đã phê duyệt
```
