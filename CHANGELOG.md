# Changelog

Tài liệu này ghi nhận toàn bộ lịch sử thay đổi, tiến độ phát triển và timeline thực tế của dự án **Standalone Policy Engine**.

Phân loại thay đổi:
*   `Added`: Các tính năng mới được phát triển.
*   `Changed`: Các thay đổi cấu trúc hoặc tối ưu hóa mã nguồn hiện có.
*   `Fixed`: Sửa các lỗi cú pháp, logic hoặc bảo mật.
*   `Security`: Các bản vá và cơ chế bảo mật hệ thống.

---

## [1.2.0] - 2026-07-04: Policy Simulation, Edge Storage (BadgerDB) & RAM GC
Hiện thực hóa các tính năng nâng cao giúp hệ thống trở nên thông minh và tự phục hồi: API giả lập chính sách không ảnh hưởng production, bộ lưu trữ cục bộ BadgerDB hỗ trợ khởi động siêu tốc khi mất mạng, và cơ chế tự động dọn dẹp RAM.

### Added
*   **Policy Simulation API (FR-010):** Thêm endpoint `POST /api/v1/tenants/{tenant_id}/simulate` vào Control Plane cho phép Admin gửi thử nghiệm bất kỳ tập chính sách DSL DRAFT nào và nhận ngay kết quả quyết định (ALLOW/DENY) kèm giải thích chi tiết, hoàn toàn không ảnh hưởng đến bộ nhớ RAM Engine đang phục vụ thật.
*   **BadgerDB Edge Storage ([badger.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger.go)):** Tầng lưu trữ cục bộ nhúng (Embedded KV) giúp PDP Sidecar khởi động siêu tốc khi không kết nối được PostgreSQL. Sau mỗi lần đồng bộ thành công, snapshot JSON tập chính sách được ghi xuống BadgerDB cục bộ. Khi khởi động, PDP tự động nạp từ BadgerDB nếu Postgres chưa có mặt.
*   **Tenant Active Cache GC ([engine_gc.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_gc.go)):** Goroutine chạy ngầm định kỳ quét dọn dẹp Trie của các Tenant không có hoạt động CheckPermission trong quá 24 giờ, giải phóng bộ nhớ RAM tự động. Kết hợp với Lazy Loading tự động tải lại từ Postgres khi có request mới đến Tenant đã bị unload.
*   **Unit Tests:** Bộ kiểm thử [badger_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger_test.go) và [engine_gc_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_gc_test.go) kiểm chứng toàn bộ: BadgerDB CRUD, GC unload idle tenant, Lazy Loading và Concurrent Safety với race detector.

---

## [1.1.0] - 2026-07-04: Observability, Performance & Cloud-Native Deployments
Hoàn thiện toàn bộ các cấu phần giám sát hệ thống, đóng gói container và manifests triển khai hạ tầng Kubernetes cluster.

### Added
*   **Prometheus Metrics:** Tích hợp bộ chỉ số đo đạc hiệu năng thời gian thực [metrics.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/metrics/metrics.go) và expose endpoint `/metrics` trên HTTP Server.
*   **gRPC Trace Interceptor:** Thiết lập Unary Interceptor trích xuất trace context (W3C standard) trong `grpc_server.go` kết nối trace liên tục từ PEP sang PDP.
*   **Docker Containerization:** Viết các Dockerfile multi-stage build tối ưu cho cả [pdp-server](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/docker/Dockerfile.pdp) và [control-plane](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/docker/Dockerfile.control).
*   **Envoy L7 Proxy Configuration:** Xây dựng file cấu hình [envoy.yaml](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/envoy/envoy.yaml) cân bằng tải HTTP/2 gRPC.
*   **Kubernetes Manifests:** Tạo file deployment [pdp-deployment.yaml](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/kubernetes/pdp-deployment.yaml) chạy sidecar Envoy proxy và [control-plane-deployment.yaml](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/kubernetes/control-plane-deployment.yaml).
*   **Performance Benchmark:** Viết kịch bản kiểm thử hiệu năng [performance_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/performance_test.go) giả lập 1,000 chính sách đo đạc độ trễ và throughput.

---

## [1.0.0] - 2026-07-04: Core Decisions & Infrastructure
Dự án được khởi tạo và hoàn thiện toàn bộ tầng logic cốt lõi (RAM Core Engine) và tầng hạ tầng phân phối dữ liệu (gRPC, PostgreSQL, Redis, Async Ring Buffer Logs).

### Added
#### Sprint 3: gRPC, DB, Sync & Logs (Hạ tầng & Vận hành)
*   **Protobuf Contract:** Định nghĩa hợp đồng gRPC v1 [policy.proto](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto) cung cấp 2 dịch vụ chính `CheckAccess` và `ExplainDecision`.
*   **gRPC Server:** Hiện thực hóa máy chủ gRPC Data Plane [grpc_server.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/grpc_server.go) tích hợp Keepalive HTTP/2 để duy trì kết nối persistent siêu tốc giữa API Gateway (PEP) và Engine (PDP).
*   **HTTP Control Plane:** Xây dựng máy chủ HTTP API quản trị [http_server.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/http_server.go) cho phép CRUD chính sách ở dạng bản thảo `DRAFT`, và endpoint `/publish` để biên dịch, kiểm tra an toàn và kích hoạt chính sách.
*   **PostgreSQL Storage:** Xây dựng tầng lưu trữ bền vững [postgres.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/postgres.go) sử dụng `pgxpool`. Hỗ trợ tự động chạy DDL Schema khởi tạo các bảng và index khi chạy, quản lý lịch sử phiên bản chính sách, và hỗ trợ ghi log bằng CopyFrom tối ưu.
*   **Redis Cache Sync:** Phát triển luồng đồng bộ hóa bộ nhớ đệm [sync.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/sync.go) qua Redis Pub/Sub đảm bảo hot reload chính sách không downtime (<300ms) kèm cơ chế Polling dự phòng mỗi 10 giây khi Redis sập.
*   **Async Audit Logs & Spill-to-Disk:** Xây dựng bộ ghi log kiểm toán bất đồng bộ [logger.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go) dùng Ring Buffer (Go channel). Tự động chuyển đổi sang ghi log cục bộ (Spill-to-Disk SSD) khi DB quá tải hoặc ngắt kết nối và tự động Replay log vào Postgres khi kết nối được khôi phục.
*   **Entrypoints:** Tạo các file main.go chạy máy chủ PDP [cmd/pdp-server/main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go) và Control Plane [cmd/control-plane/main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/control-plane/main.go).

#### Sprint 2: In-Memory Index Trie & AST Evaluator (RAM Core Engine)
*   **Multi-level Index Trie:** Xây dựng cấu trúc lưu trữ RAM Trie [trie.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go) phân cấp giúp tra cứu chính sách đạt độ phức tạp $O(\log N)$ thay vì duyệt tuyến tính $O(N)$.
*   **Global Rules Partitioning:** Phân tách các chính sách wildcard kép (`principal == any && resource == any`) ra phân vùng riêng để tránh ô nhiễm chỉ mục Trie.
*   **Role Hierarchy Resolving (DAG):** Xây dựng đồ thị vai trò [dag.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go) có thuật toán DFS phát hiện chu trình đệ quy và tính toán trước Transitive Closure cho phép kiểm tra quan hệ vai trò với độ phức tạp $O(1)$ ở runtime.
*   **AST Evaluator:** Hiện thực bộ đánh giá AST [evaluator.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go) hỗ trợ đoản mạch logic (short-circuit), so khớp bitwise IP, so khớp DateTime bằng số nguyên, và tích hợp `sync.Pool` tái sử dụng context.
*   **Decision Logic:** Phát triển thuật toán ra quyết định [decision.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/decision.go) dựa trên quy tắc mặc định cấm (Deny-by-Default), luật cấm ghi đè (Forbid Overrides), và tính năng giải thích quyết định (Policy Explain).
*   **Copy-On-Write (COW):** Triển khai cơ chế nhân bản Trie và hoán đổi con trỏ nguyên tử (Atomic Pointer Swap) trong [engine.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine.go) giúp luồng đọc luôn chạy tự do lock-free.
*   **Unit & Concurrency Tests:** Viết các bài test đa luồng đọc ghi đồng thời với cờ `-race` trong [engine_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_test.go).

#### Sprint 1: Policy Language Compiler & AST Parser (DSL & Compiler)
*   **AST Nodes:** Định nghĩa các node biểu diễn cây AST bất biến trong [ast.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/ast.go).
*   **Stateful Lexer:** Hiện thực hóa bộ tokenizer [lexer.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/lexer.go) có khả năng chuyển đổi trạng thái khi gặp dấu `{` để giải quyết sự nhập nhằng của từ khóa Scope tĩnh và Variable động.
*   **Pratt Parser:** Xây dựng trình phân tích cú pháp đệ quy đi xuống [parser.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/parser.go) dựa trên độ ưu tiên của toán tử.
*   **AST Compiler & Optimizations:** Thiết lập trình biên dịch [compiler.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/compiler.go) thực hiện Constant Folding, IP/DateTime Pre-parsing sang dạng nhị phân/số nguyên, và kiểm tra kiểu dữ liệu tĩnh.

### Security
*   **AST Depth Limit:** Chặn biên dịch chính sách nếu độ sâu biểu thức logic lồng nhau vượt quá **15 cấp** để phòng chống tấn công DoS tràn stack.
*   **Safe Missing Attributes:** Thuộc tính thiếu trong ngữ cảnh request được evaluator chuyển về trạng thái `ERROR` và trả về `false` (Fail-closed) một cách an toàn mà không gây panic hệ thống.
