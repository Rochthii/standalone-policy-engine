# Standard Go Project Layout

Tài liệu này đặc tả cấu trúc thư mục mã nguồn (Project Layout) của dự án **Standalone Policy Engine** tuân thủ theo chuẩn **Standard Go Project Layout** (`golang-standards/project-layout`).

---

## 1. Sơ đồ Cấu trúc Thư mục (Directory Tree)

```text
standalone-policy-engine/
├── api/                  # Định nghĩa Protobuf (.proto) và OpenAPI specs
│   └── proto/
│       └── policy.proto
├── cmd/                  # Điểm chạy chính (Entrypoints) cho các ứng dụng
│   ├── pdp-server/       # Khởi chạy PDP gRPC Server (Data Plane)
│   │   └── main.go
│   └── control-plane/    # Khởi chạy Control Plane HTTP API (Control Plane)
│       └── main.go
├── internal/             # Mã nguồn riêng tư (Private code), không cho phép bên ngoài import
│   ├── parser/           # Lexer, Parser & AST Compiler cho DSL
│   │   ├── lexer.go
│   │   ├── parser.go
│   │   └── ast.go
│   ├── engine/           # Động cơ quyết định (Trie Indexer & AST Evaluator)
│   │   ├── trie.go
│   │   ├── evaluator.go
│   │   └── decision.go
│   ├── server/           # gRPC và HTTP Server handlers
│   │   ├── grpc_server.go
│   │   └── http_server.go
│   ├── storage/          # Tầng tương tác DB (PostgreSQL, BadgerDB client)
│   │   ├── postgres.go
│   │   └── badger.go
│   └── audit/            # Ring Buffer & Async Logging
│       └── logger.go
├── pkg/                  # Mã nguồn công khai (Public libraries), các service khác có thể import
│   └── client/           # gRPC Client SDK giúp API Gateway/PEP tích hợp nhanh
│       └── client.go
├── deployments/          # Cấu hình triển khai hạ tầng (K8s Helm, Docker Compose)
│   ├── docker/
│   │   ├── Dockerfile.pdp
│   │   └── Dockerfile.control
│   └── helm/
│       └── policy-engine/
├── scripts/              # Các script helper (Compile proto, run test, setup db)
│   ├── compile_proto.sh
│   └── setup_db.sql
├── tests/                # Thư mục chứa các bài kiểm thử tích hợp & stress test
│   ├── integration_test.go
│   ├── benchmark_test.go
│   └── security_test.go
├── docs/                 # Tài liệu thiết kế kiến trúc doanh nghiệp
│   ├── requirements/     # Yêu cầu hệ thống
│   ├── domain/           # Mô hình thực thể
│   ├── policy-language/  # Đặc tả ngôn ngữ chính sách
│   ├── evaluation-engine/# Thiết kế core engine
│   ├── architecture/     # Kiến trúc hệ thống
│   ├── security/         # Thiết kế bảo mật chuyên sâu
│   ├── performance/      # Phân tích hiệu năng, đo đạc latency
│   ├── data/             # Thiết kế database schema
│   ├── api/              # Định nghĩa API gRPC & REST
│   ├── roadmap/          # Lộ trình & Sprints
│   ├── project-structure/# Bố trí thư mục dự án Go
│   ├── testing/          # Chiến lược kiểm thử tự động
│   ├── operations/       # Runbooks, giám sát & SLOs
│   └── adr/              # Architecture Decision Records
├── go.mod
└── go.sum
```
---
*Ghi chú: Việc phiên bản hóa toàn bộ tài liệu thiết kế nằm ngay trong thư mục `/docs` giúp các thành viên phát triển có thể tham chiếu trực tiếp ngay trong quá trình viết code.*
