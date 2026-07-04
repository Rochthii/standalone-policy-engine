# Standalone Policy Engine - Master Index

Tài liệu này là sơ đồ mục lục tổng thể điều hướng toàn bộ hệ thống tài liệu thiết kế kiến trúc doanh nghiệp của dự án **Standalone Policy Engine**.

---

## 🗺️ Bản Đồ Tài Liệu Thiết Kế (Documentation Map)

```text
docs/
├── 00_MASTER_INDEX.md             # Tài liệu này (Mục lục tổng thể)
├── requirements/
│   └── requirements.md            # Đặc tả 10 Yêu cầu Chức năng (FR) & Phi chức năng (NFR)
├── domain/
│   └── domain-model.md            # Đặc tả 10 Thực thể cốt lõi & Sơ đồ ERD
├── policy-language/
│   ├── policy-language-overview.md# Triết lý thiết kế DSL
│   ├── grammar-specification.md   # Đặc tả ngữ pháp EBNF & Token Parser (Core Spec 1)
│   ├── syntax-reference.md        # Hướng dẫn cú pháp toán tử & kiểu dữ liệu
│   ├── semantic-rules.md          # Quy tắc ngữ nghĩa & Thừa kế vai trò
│   ├── evaluation-model.md        # Thuật toán quyết định (Deny-by-Default)
│   └── examples.md                # Các ví dụ thực tế về RBAC, ABAC, PBAC
├── evaluation-engine/
│   ├── ast-design.md              # Cấu trúc Node AST trong bộ nhớ RAM
│   ├── parser-design.md           # Thiết kế Lexer & Parser đệ quy đi xuống
│   ├── compiler-design.go.md      # Tối ưu hóa compile và nén IP/DateTime
│   ├── execution-engine.md        # Thuật toán Radix Trie & Copy-On-Write (Core Spec 2)
│   ├── decision-model.md          # Chi tiết bộ giải thích quyết định (Explain)
│   └── caching-strategy.md        # Đồng bộ hóa cache động qua Redis Pub/Sub
├── architecture/
│   ├── system-overview.md         # Tổng quan luồng PEP-PDP
│   ├── logical-architecture.md    # Sơ đồ khối các module logic nội bộ
│   ├── physical-architecture.md   # Quản lý luồng Go-routine & Lock Mutex
│   ├── deployment-architecture.md # Topology Cluster & Sidecar với Envoy L7
│   ├── data-flow.md               # Luồng nạp chính sách và đánh giá quyền
│   └── sequence-diagrams.md       # Sơ đồ tuần tự bắt tay gRPC
├── security/
│   ├── security-overview.md       # Trust boundaries & mTLS controls
│   ├── policy-tampering.md        # Phòng chống sửa đổi chính sách qua chữ ký ED25519
│   ├── tenant-isolation.md        # Cách ly logic & RAM đa khách thuê
│   └── authorization-bypass.md    # Fail-closed design và chống bypass gRPC
├── performance/
│   ├── latency-budget.md          # Phân bổ ngân sách độ trễ toàn trình < 1ms
│   ├── memory-model.md            # Stack/Heap allocations & tối ưu GC pool
│   └── benchmark-matrix.md        # Ma trận kiểm thử tải từ 10 đến 100k policies
├── data/
│   └── data-model.md              # Schema PostgreSQL, BadgerDB & WORM logs
├── api/
│   ├── decision-api.md            # gRPC CheckAccess & REST POST decisions
│   ├── management-api.md          # HTTP API quản trị CRUD chính sách
│   └── explain-api.md             # gRPC ExplainDecision API
├── roadmap/
│   └── roadmap.md                 # Lộ trình 9 Phases & Kế hoạch chia Sprint
├── project-structure/
│   └── project-layout.md          # Bố trí thư mục Standard Go Layout
├── testing/
│   └── testing-strategy.md        # Chiến lược Unit/Fuzz/Integration/Chaos test
├── operations/
│   └── runbooks.md                # Giám sát SLOs & cẩm nang cứu hộ sự cố
└── adr/
    └── adr-001-policy-engine-design.md # Quyết định kiến trúc cốt lõi đã phê duyệt
```
