# Changelog

Tài liệu này ghi nhận toàn bộ lịch sử thay đổi, tiến độ phát triển và timeline thực tế của dự án **Standalone Policy Engine**.

Phân loại thay đổi:
*   `Added`: Các tính năng mới được phát triển.
*   `Changed`: Các thay đổi cấu trúc hoặc tối ưu hóa mã nguồn hiện có.
*   `Fixed`: Sửa các lỗi cú pháp, logic hoặc bảo mật.
*   `Security`: Các bản vá và cơ chế bảo mật hệ thống.

---

## [1.4.0] - 2026-07-05: High-Performance CLI Tool (`pectl`) & Automation Support

Bổ sung công cụ dòng lệnh (CLI) `pectl` chuẩn production-grade hỗ trợ quản trị chính sách, giả lập và ra quyết định thông qua REST Control Plane.

### Added
- **`pectl` CLI Tool (`cmd/pectl`):** Khởi tạo ứng dụng CLI sử dụng `cobra` và `viper`, hỗ trợ đa dạng câu lệnh quản trị vòng đời chính sách, quản lý tenant, đo đạc telemetry và kiểm tra sức khỏe hệ thống.
- **Enterprise-Grade Client (`internal/pectl/client.go`):** Reusable HTTP API client với cơ chế tự động gửi lại yêu cầu (retry với exponential backoff 100ms-2s) tối đa 3 lần cho lỗi 5xx, hỗ trợ xử lý lỗi có cấu trúc theo chuẩn RFC 7807 (Problem Details).
- **Tabular & Structured Printing (`internal/pectl/printer`):** Triển khai tầng xuất dữ liệu ra Console hỗ trợ 3 chế độ `--output`: `table` (sử dụng tabwriter căn lề cột), `json` (định dạng đẹp) và `yaml` (gopkg.in/yaml.v3).
- **Flexible Configuration:** Ưu tiên cấu hình động theo thứ tự: CLI Flags > Environment Variables (`PECTL_*`) > File cấu hình (`~/.pectl/config.yaml`).
- **Commands Added:**
  - `policy`: `create`, `update`, `publish`, `delete`, `list`, `get`
  - `simulate`: Giả lập quyết định với ngữ cảnh JSON (`--context-file`), nạp chính sách draft cục bộ (`--draft-file`) và gộp chính sách active (`--include-active`).
  - `check` & `explain`: Kiểm tra quyền truy cập trực tiếp kèm đo lường latency chính xác và hiển thị vết thực thi (trace).
  - `tenant`: `list`, `get`, `status`
  - `metrics` & `health`: Thu thập telemetry (latency P50/P95/P99, QPS, GC) và kiểm tra sức khỏe thành phần hệ thống.
- **Setup Scripts & Makefile Targets:**
  - Viết script tự động cài đặt `go mod tidy` và biên dịch nhanh `setup-pectl.sh` (Linux/WSL) và `setup-pectl.ps1` (Windows).
  - Thêm target `build-pectl`, `install-pectl`, `test-pectl`, và `tidy` vào `Makefile`.
- **Unit Tests:** Kiểm thử 100% logic nạp cấu hình, cơ chế retry/error client và định dạng đầu ra của printer.

---

## [1.3.0] - 2026-07-04: JWT Token Validation, AES-GCM Log Encryption, Redis Universal Client & Sprint 7 Final Deliverables

Sprint cuối cùng & Sprint 7. Hoàn thiện tầng bảo mật, vận hành phân tán, dọn dẹp bộ nhớ RAM tự động, cấu trúc cơ sở dữ liệu có phiên bản, đo đạc hiệu năng và kiểm thử tích hợp E2E:

### Added
*   **E2E Integration Test ([e2e_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_test.go)):** Xây dựng luồng kiểm thử E2E trong container qua Docker Compose kết nối PostgreSQL và Redis thực tế. Tự động khởi tạo Tenant ngẫu nhiên trong DB, thực hiện CRUD chính sách, kiểm thử phân quyền gRPC, giả lập mất kết nối Redis để kiểm thử đồng bộ dự phòng (Fallback Polling).
*   **JSON Codec for Mock gRPC ([policy.pb.go](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.pb.go)):** Đăng ký JSON codec cho gRPC giúp đóng gói và giải tuần tự các struct viết tay (mock protobuf) qua mạng TCP Docker mà không cần cài đặt trình biên dịch `protoc` trên máy local.
*   **Database Migration System ([migrations](file:///e:/Projects/Project_TN/standalone-policy-engine/db/migrations/)):** Tách biệt DDL schema cứng và thay bằng hệ thống migrations có phiên bản sử dụng `golang-migrate/migrate/v4`. Tự động khởi chạy migration khi khởi động tầng PostgreSQL Storage.
*   **Audit Logger Graceful Degradation ([logger.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go)):** Cấu hình exponential back-off (1 giây đến tối đa 1 phút) cho replay worker. Thêm cơ chế giới hạn thư mục log cục bộ tối đa 1GB (Spill-to-Disk size limit) để bảo vệ đĩa cứng tránh bị tràn.
*   **GC Configuration via Env Vars ([main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go)):** Hỗ trợ cấu hình RAM GC động thông qua các biến môi trường `GC_ENABLED`, `GC_INTERVAL`, `GC_IDLE_TIMEOUT` để tối ưu hóa bộ nhớ đệm in-memory trie.
*   **Benchmark Validation ([benchmark_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/benchmark_test.go)):** Bổ sung bộ kiểm thử tải trọng lớn đo đạc throughput và latency của PDP Engine dưới các mức quy mô 1k, 10k, 100k chính sách và lưu trữ kết quả tại [results.md](file:///e:/Projects/Project_TN/standalone-policy-engine/benchmarks/results.md).
*   **JWT Token Validation ([jwt.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/jwt.go)):** PDP gRPC Server tự động trích xuất và xác thực JWT Token (HMAC-SHA256) từ gRPC Metadata `authorization`. Claims được parse và nạp vào `req.Subject` và `req.Context` trước khi chạy bộ đánh giá ABAC. PEP không cần giải mã token thủ công.
*   **AES-GCM Envelope Encryption ([crypto.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/crypto.go)):** Mọi trường nhạy cảm trong log kiểm toán (subject, action, resource, context) được mã hóa bằng AES-GCM 256-bit trước khi ghi vào PostgreSQL hoặc Spill-to-Disk. Mỗi bản ghi dùng một DEK ngẫu nhiên riêng, DEK được mã hóa bởi KEK từ biến môi trường `LOG_KEK`. Ngay cả admin PostgreSQL cũng không thể đọc nội dung log.
*   **Redis Universal Client:** Cả `cmd/pdp-server` và `cmd/control-plane` hỗ trợ ba chế độ kết nối Redis thông qua biến môi trường `REDIS_MODE`: `single` (mặc định), `sentinel` (Failover) và `cluster` (Horizontal Scale). Không cần sửa code khi nâng cấp topology Redis.
*   **PDP Node Heartbeat Registry:** `Syncer.heartbeatWorker` định kỳ 5 giây gửi JSON heartbeat kèm node ID, trạng thái và số Tenant đang hoạt động lên kênh Redis `pdp-heartbeats`. Cho phép Control Plane theo dõi số lượng và sức khỏe tất cả node PDP trong cluster.
*   **Unit Tests ([jwt_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/jwt_test.go), [crypto_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/crypto_test.go)):** Kiểm thử toàn diện cả hai module: JWT validate hợp lệ/hết hạn/sai secret, Bearer prefix stripping, Envelope Encrypt/Decrypt vòng đời, nonce độc lập, DEK sai và payload >5KB.

### Fixed
*   **gRPC Tenant Isolation Mismatch:** Khắc phục lỗi so khớp Subject trong E2E test bằng cách đồng bộ hóa JWT token đăng nhập của Alice/Bob khớp với danh tính gửi đi, ngăn chặn bộ lọc Interceptor chặn nhầm request.
*   **Nil Response Handling:** Ngăn ngừa lỗi Panic bằng cách bọc kiểm tra an toàn `nil` cho các response HTTP trước khi gọi close body trong E2E tests khi container Redis bị tắt.

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
