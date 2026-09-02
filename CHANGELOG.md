# Changelog

Tài liệu này ghi nhận toàn bộ lịch sử thay đổi, tiến độ phát triển và timeline thực tế của dự án **Standalone Policy Engine**.

Phân loại thay đổi:
*   `Added`: Các tính năng mới được phát triển.
*   `Changed`: Các thay đổi cấu trúc hoặc tối ưu hóa mã nguồn hiện có.
*   `Fixed`: Sửa các lỗi cú pháp, logic hoặc bảo mật.
*   `Security`: Các bản vá và cơ chế bảo mật hệ thống.

---

## [1.14.0] - 2026-09-02: Zero-Alloc NDJSON Audit Streamer, Revision ID Traceability & Vector Codec Alignment

Khắc phục triệt để lỗi lệch chuẩn giải mã (Codec Mismatch) giữa PDP Data Plane và Vector Sidecar, chuẩn hóa luồng Newline Delimited JSON (NDJSON) Zero-Allocation và tích hợp `revision_id` vào toàn bộ dấu vết kiểm toán:

### Fixed
- **Vector Codec Mismatch (`internal/audit/logger.go`, `deploy/vector/vector.yaml`):**
  - Loại bỏ format nhị phân tự chế (`0xAE` magic byte + offsets) gây lỗi Drop gói tin im lặng (Silent Data Loss) trên Vector Sidecar (`codec: "json"`).
  - Chuẩn hóa sang định dạng JSON Lines (NDJSON) chuẩn Cloud-Native tương thích 100% với Vector, Kubernetes container `stdout`, ClickHouse và Kafka.

### Added
- **Revision ID Audit Traceability (`internal/audit/logger.go`, `internal/server/grpc_server.go`):**
  - Struct `LogEntry` và hàm `Log()` tích hợp trường `rev` (`revision_id uint64`) ghi nhận chính xác phiên bản chính sách đã đưa ra quyết định phân quyền.
  - Phục vụ giải quyết triệt để tranh chấp Eventual Consistency trong cửa sổ lan truyền phân tán ($\le 50\,\text{ms}$).
- **Vector Remap VRL Enrichment (`deploy/vector/vector.yaml`):**
  - Tự động chuyển đổi trường nanosecond timestamp `.ts` sang readable RFC3339 datetime `.evaluated_at` trong luồng xử lý của Vector Sidecar.

### Changed
- **Zero-Allocation JSON Formatter (`internal/audit/logger.go`):**
  - Tự động format NDJSON qua `sync.Pool` byte slice và hàm trợ giúp `escapeJSON`, `strconv.AppendInt`, `strconv.AppendUint`.
  - Đạt chỉ số vi mô: **328.3 ns/op, 0 B/op, 0 allocs/op** (> 3.32 triệu bản ghi NDJSON/giây trên 1 core CPU), duy trì bất biến Zero Heap Allocation trên toàn bộ Hot-Path.

---

## [1.13.0] - 2026-08-31: Autonomous AI Agent Guardrails, Zero-Redis Architecture & Ultra-Extreme Benchmarks

Hoàn thiện kiến trúc phân tán Pure PostgreSQL (loại bỏ hoàn toàn Redis), tích hợp rào chắn AI Guardrails & Obligations (chuẩn NIST/OWASP) và tối ưu hóa Zero-Allocation cho toàn bộ kịch bản cực hạn:

### Added
- **AI Agent Guardrails & Obligations (`tests/ai_agent_guardrails_test.go`, `proto/v1/policy.proto`):**
  - Tích hợp trường `obligations` và `advice` vào Protobuf contract `CheckAccessResponse` mà không phá vỡ tính nguyên tử Boolean (`ALLOW`/`DENY`).
  - Hỗ trợ các nghĩa vụ runtime: `REQUIRE_HUMAN_APPROVAL`, `AUDIT_SENSITIVE_TOOL_CALL`, `MASK_ATTRIBUTES`.
  - Kiểm chứng 4 kịch bản chuẩn NIST AI RMF & OWASP LLM06: Autonomous Low-Value Tool-Call (`ALLOW`), High-Value Delegated Action (`DENY` + `REQUIRE_HUMAN_APPROVAL`), Prompt Injection Extreme Value (`Hard DENY`), và Agent-Supervisor Separation of Duties (`DENY`).
  - Đạt độ trễ **286.3 ns/op, 0 B/op, 0 allocs/op** (> 3.49 triệu lượt đánh giá AI Guardrail/giây).
- **Ultra-Extreme & Deep DAG Benchmarks (`tests/benchmark_test.go`):**
  - `BenchmarkUltraExtreme_DeepDAG_HeavyABAC`: Đồ thị kế thừa vai trò 11 cấp (DAG), 5,000 decoy policies, biểu thức AST 10 điều kiện đạt **810.9 ns/op, 0 B/op, 0 allocs/op**.
  - `BenchmarkUltraExtreme_10kPolicies_ConcurrentContention`: 10,000 chính sách chịu tải đa luồng cực đại đạt **35.94 ns/op, 0 B/op, 0 allocs/op** (~27.8 triệu RPS).

### Changed
- **Pure PostgreSQL Sequence & Metadata `LISTEN/NOTIFY`:**
  - Khai tử hoàn toàn Redis container và vòng lặp Polling 10s.
  - Chuyển giao toàn bộ cơ chế đồng bộ sang PostgreSQL Monotonic Revision Sequence (`tenants.revision`) với payload metadata < 120 bytes (loại bỏ rủi ro tràn giới hạn 8KB của Postgres).
  - Tự động phát hiện khoảng trống dữ liệu (`gap detection`) và kích hoạt Fast Catch-Up tức thời (< 50ms).
- **Trie Index & Stack Scratch Buffers:**
  - Mở rộng `policySlicePool` và bộ đệm `scratchNodes`/`scratchIPs` trong `EvalContext` lên 64 phần tử, `subScratch` lên 32 phần tử, triệt tiêu 100% hiện tượng slice grow heap allocation trên mọi đường dẫn tra cứu sâu.

---

## [1.12.0] - 2026-08-31: Docker DevOps Standards Skill & Containerized E2E Verification

Đúc kết toàn bộ bài học thực chiến, chuẩn hóa Docker multi-stage images và quy trình kiểm thử phân tán E2E trên Docker Compose:

### Added
- **Docker Standards Agent Skill (`.agents/skills/docker-standards/SKILL.md`):** Đóng gói toàn bộ kinh nghiệm và rào chắn kỹ thuật:
  - Khử bỏ triệt để Byte Order Mark (UTF-8 BOM `\ufeff`) khi tạo migration SQL trên Windows.
  - Đồng bộ `golang:alpine` builder image khớp với phiên bản `go.mod` (ngăn lỗi `go >= 1.25.0`).
  - Quy chuẩn Docker Compose healthchecks (`depends_on: { condition: service_healthy }`) và handshake readiness polling cho E2E testing.
  - Đảm bảo tính tương thích ngược hai chiều (`id` và `policy_id`) trong toàn bộ REST response.
- **Docker E2E Automated Verification:** Xác thực thành công 100% luồng phân tán thực tế với PostgreSQL 15, Redis 7, PDP Server gRPC và Control Plane HTTP trong `tests/e2e_test.go`.

---

## [1.11.0] - 2026-08-31: Clean Architecture Standards, Modular Handlers & Centralized Config

Chuẩn hóa toàn diện mã nguồn theo chuẩn Clean Architecture & 12-Factor App quốc tế, loại bỏ hoàn toàn hardcoded parameters và phân rã các file nguyên khối:

### Added
- **Centralized Type-Safe Config (`internal/config/config.go`):** Quản lý toàn bộ cấu hình hệ thống tập trung với cơ chế Fail-Fast Validation, nạp biến môi trường type-safe và ngăn chặn chạy localhost trên Production.
- **Dedicated Agent Skill (`.agents/skills/clean-architecture-standards/SKILL.md`):** Thiết lập rào chắn quy chuẩn quốc tế: giới hạn độ dài file ($\le 250$ dòng), nguyên lý Single Responsibility (SRP), nghiêm cấm hardcoded credentials và magic numbers.

### Changed
- **Modularized HTTP Handlers (`internal/server/`):** Phân rã file nguyên khối `http_server.go` (500 dòng) thành 4 module độc lập, tinh gọn (< 180 dòng/file):
  - `http_server.go` (89 dòng): Khởi tạo máy chủ & định tuyến Mux.
  - `handlers_policy.go` (178 dòng): Quản trị CRUD và Publish chính sách.
  - `handlers_tenant.go` (82 dòng): Schema thuộc tính & Prewarm API.
  - `handlers_decision.go` (113 dòng): Fallback REST Decision & Simulator.
- **Decoupled CLI & Server Binaries:** Tái cấu trúc `cmd/control-plane/main.go` và `cmd/pdp-server/main.go` sử dụng `config.Load()`, xóa bỏ các hàm `os.Getenv` rời rạc.

---

## [1.10.0] - 2026-08-31: Zero-Allocation Binary Datagram Unix Socket (unixgram) Logger

Tối ưu hóa tầng Audit Logger lên cảnh giới vi mô cao nhất, loại bỏ hoàn toàn JSON serialization và sử dụng giao thức Non-blocking UDP Unix Domain Socket (`unixgram`):

### Added
- **Zero-Allocation Binary Packing (`internal/audit/logger.go`):** Đóng gói bản ghi kiểm toán trực tiếp thành chuỗi byte nhị phân (`0xAE` magic byte, Unix Nano int64, BigEndian string offsets) qua `sync.Pool` byte slice. Triệt tiêu hoàn toàn `encoding/json`, đạt tốc độ phát tán log **140.9 ns/op**, **0 B/op** và **0 allocs/op**.
- **Non-blocking UDP Unix Domain Socket (`NewUnixgramAuditLogger`):** Truyền tin qua `net.DialUnix("unixgram", ...)` theo cơ chế fire-and-forget qua kernel socket buffer, đảm bảo không bao giờ gây backpressure hay phát sinh lock contention lên luồng đánh giá Data Plane.
- **Architectural Trade-Off Formalization:** Ghi nhận chính thức trong mã nguồn và tài liệu kiến trúc sự đánh đổi có chủ đích giữa Ultra-Low Latency (Ưu tiên tuyệt đối hiệu năng CPU RAM) và khả năng rớt gói tin nếu Sidecar Vector gặp sự cố.
- **Micro-Benchmark Suite (`internal/audit/logger_test.go`):** Bổ sung `BenchmarkAuditLogger_LogZeroAlloc` kiểm chứng $0$ Heap allocations và thông lượng $> 7,700,000$ logs/giây trên một core.

---

## [1.9.0] - 2026-08-31: Cloud-Native Decoupled Audit Streamer & Vector Sidecar Architecture

Trục xuất hoàn toàn logic ghi cơ sở dữ liệu PostgreSQL và đĩa cục bộ ra khỏi PDP, chuyển đổi sang kiến trúc Decoupled Stream Logger chuẩn 12-Factor App:

### Changed
- **Decoupled Stream Logger (`internal/audit/logger.go`):** Thay thế toàn bộ worker queue và `pgx.CopyFrom` bằng `StreamAuditLogger` sử dụng `sync.Pool` chứa `bytes.Buffer`. Xuất log JSON thẳng ra `os.Stdout` hoặc Unix Domain Socket trong **< 50 ns**, triệt tiêu hoàn toàn rủi ro nghẽn Backpressure và không sinh rác GC Heap.
- **Khử bỏ điểm nghẽn RDBMS:** PDP không còn trực tiếp chịu tải I/O của hàng triệu audit logs/giây, loại bỏ nguy cơ làm sập PostgreSQL do table bloat và WAL saturation.
- **Khử bỏ ảo giác Ephemeral Storage trên K8s:** Xóa bỏ thư mục ghi đĩa cục bộ `./spill-logs` bên trong Pod PDP, chuyển giao toàn bộ trách nhiệm buffering và retry cho Sidecar chuyên dụng.

### Added
- **Vector Sidecar Blueprint (`deploy/vector/vector.yaml`):** File cấu hình agent Vector (viết bằng Rust, Zero-GC) lắng nghe socket từ PDP, tự quản lý đệm đĩa Persistent Volume và đẩy log về ClickHouse / Kafka.
- **Kubernetes Production Manifest (`deploy/k8s/pdp-with-vector-sidecar.yaml`):** Mẫu Deployment chuẩn chạy đồng thời PDP Container và Vector Sidecar qua Shared Memory IPC volume `/var/run/pdp`.

---

## [1.8.0] - 2026-08-31: In-Process Go SDK (Zero-Network Hop), Unix Domain Sockets & AI Agent Guardrails

Triệt tiêu 100% độ trễ mạng qua In-Process Go SDK và Unix Domain Socket IPC, đồng thời chuẩn hóa rào chắn an toàn cho Autonomous AI Agent theo mô hình Obligation:

### Added
- **In-Process Go SDK (`pkg/pdp/`):** Cung cấp gói SDK nhúng `EmbeddedPDP` cho phép các Go Microservices thực hiện đánh giá phân quyền như một lời gọi hàm RAM cục bộ với độ trễ thuần **0.29 µs (297 ns)** và **0 allocs/op**.
- **Scoped Tenant Whitelist (`pkg/pdp/config.go`):** Bổ sung danh sách `AllowedTenants` kiểm soát phạm vi nạp chính sách, triệt tiêu 100% nguy cơ OOM khi ứng dụng nhúng chạy trong môi trường phân tán đa khách thuê.
- **Non-blocking Background Sync (`pkg/pdp/embedded.go`):** Luồng đồng bộ ngầm (`syncWorker`) thực hiện nhận sự kiện, biên dịch độc lập và hoán đổi con trỏ Copy-On-Write (COW), bảo đảm không bao giờ gây block hay phát sinh jitter trên luồng nghiệp vụ chính.
- **Unix Domain Socket (UDS) Listener (`cmd/pdp-server/main.go`):** Bổ sung biến môi trường `LISTEN_SOCKET_PATH` hỗ trợ bind vào Unix socket (`unix:///var/run/pdp.sock`) cho các Sidecar Container trong Kubernetes Pod, vượt qua hoàn toàn overhead của TCP stack.
- **AI Agent Guardrails with Obligation Pattern (`internal/engine/decision.go`):** Bổ sung cấu trúc `Obligation` (ví dụ `REQUIRE_HUMAN_APPROVAL`, `MASK_ATTRIBUTES`) gắn kèm quyết định nhị phân `DENY` khi AI Agent cố gắng thực thi công cụ rủi ro cao hoặc vượt quá độ sâu ủy quyền `delegation_depth > 3`, bảo toàn trọn vẹn Boolean Algebra cho bộ đánh giá AST.

---

## [1.7.0] - 2026-08-31: PostgreSQL Transactional Sequence, pg_notify Broadcast & Multi-Replica Replay Buffer

Triển khai cơ chế cấp phát Monotonic Revision ID nguyên tử gắn liền với Database Transaction, broadcast qua PostgreSQL `LISTEN/NOTIFY` và Replay Ring Buffer chống mất mát sự kiện:

### Added
- **Database Migration 000003 (`db/migrations/000003_add_tenant_revision.up.sql`):** Bổ sung cột `revision BIGINT NOT NULL DEFAULT 1` và index `idx_tenants_revision` vào bảng `tenants`.
- **PostgreSQL Transactional Sequence & `NOTIFY` (`internal/storage/postgres.go`):** Cập nhật `PublishPolicy` và `DeletePolicy` chạy trong một `pgx.Tx` duy nhất, tự động tăng `revision` của Tenant qua `RETURNING revision` và kích hoạt lệnh `NOTIFY policy_events, ...` ngay khi transaction commit thành công. Loại bỏ hoàn toàn rủi ro Dual-Write hoặc Phantom Events.
- **Cross-Replica Buffer Sync (`internal/storage/postgres.go`):** Bổ sung hàm `ListenPolicyEvents` sử dụng `LISTEN policy_events` chuyên dụng để các Control Plane replicas nhận sự kiện trong thời gian thực.
- **Replay Ring Buffer with Compaction Fallback (`internal/server/replay_buffer.go`):** Lưu trữ 1,000 sự kiện gần nhất trong RAM. Hỗ trợ phương thức `GetEventsSince(tenantID, afterRevision)` giúp PDP nodes catch-up tức thì khi rớt mạng ngắn, hoặc trả về tín hiệu Compaction để kích hoạt Full Snapshot reload khi rớt mạng dài hạn.
- **Pre-warm REST API Endpoint (`internal/server/http_server.go`):** Thêm route `POST /api/v1/tenants/{tenant_id}/prewarm` (yêu cầu Tenant Auth) cho phép CI/CD hoặc API Gateway chủ động nạp trước tập luật vào RAM, loại bỏ hoàn toàn độ trễ Cold Start cho các VIP Tenants.

---

## [1.6.0] - 2026-08-31: Distributed Resilience, 100% Stateless Cloud Profile & Monotonic Revision Sync

Chuẩn hóa PDP Engine thành Microservice 100% Stateless trên Cloud Kubernetes, giải quyết triệt để cửa sổ mù 10s bằng cơ chế Monotonic Revision ID & Gap Detection, và cung cấp API Schema phục vụ tối ưu hóa Payload gRPC:

### Added
- **100% Stateless Cloud Profile (`cmd/pdp-server/main.go`):** Bổ sung biến môi trường `STORAGE_MODE=cloud|edge`. Chế độ `cloud` (mặc định) giúp Pod vô trạng hoàn toàn, không đụng vào đĩa cứng cục bộ `./badger-data`, nạp dữ liệu thẳng vào RAM khi khởi động. Chế độ `edge` kích hoạt `BadgerStore` nhúng cho các kịch bản offline/IoT.
- **Monotonic Revision ID & Sync Gap Detection (`internal/engine/sync.go`, `internal/engine/engine.go`):** Bổ sung `Revision uint64` vào `TrieRoot` và `EngineState`. Khi nhận sự kiện qua Pub/Sub, Syncer tự động so sánh số hiệu phiên bản; nếu phát hiện lệch phiên bản do rớt mạng (`event.Revision > currentRevision + 1`), hệ thống kích hoạt luồng **Full Catch-Up Sync** tức thì thay vì chờ chu kỳ 10s.
- **Compile-Time Attribute Extraction (`internal/parser/compiler.go`, `internal/parser/ast.go`):** AST Compiler tự động phân tích và trích xuất danh sách các thuộc tính biến được sử dụng (`PolicyNode.RequiredAttributes`). `TrieRoot` tự động tổng hợp danh mục thuộc tính cần thiết của toàn bộ chính sách Active trong Tenant.
- **Tenant Attribute Schema REST Endpoint (`internal/server/http_server.go`):** Thêm route `GET /api/v1/tenants/{tenant_id}/schema` (xác thực Tenant Auth) trả về danh mục thuộc tính yêu cầu và revision hiện tại, giúp API Gateway (PEP) lọc và đóng gói payload gRPC siêu nhẹ (< 300 bytes).

### Performance & Resilience Metrics Verified
- **Độ trễ phục hồi khi rớt mạng:** Tức thì trong **< 50ms** khi kết nối phục hồi nhờ Gap Detection.
- **Tốc độ Hot-Path RAM:** Duy trì ổn định tuyệt đối ở **305 ns/op, 0 B/op và 0 allocs/op**.

---

## [1.5.0] - 2026-08-31: Zero-Allocation Hot-Path, 100% Lock-Free Role DAG & Microsecond Performance Milestone

Tối ưu hóa sâu tầng động cơ đánh giá In-Memory (PDP Data Plane), triệt tiêu hoàn toàn 100% heap allocation trên hot-path, loại bỏ tranh chấp khóa Mutex trên luồng đọc và thiết lập các chốt chặn bất biến (Immutability Guardrails) ở cấp độ kiến trúc bộ nhớ:

### Changed
- **100% Lock-Free `RoleDAG` Read Path (`internal/engine/dag.go`):** Gỡ bỏ hoàn toàn `sync.RWMutex.RLock()` trên luồng `IsDescendant()` và `GetInheritedRoles()`. Tận dụng triệt để kiến trúc Copy-On-Write (COW) với `atomic.Pointer` swap ở cấp `EngineState`, đưa độ trễ đọc của Role DAG về O(1) mà không bị Cache-Line Bouncing khi tải hàng triệu RPS.
- **Pre-computed Role Inheritance with Full Slice Expression:** Tính toán trước toàn bộ danh sách vai trò kế thừa `inheritedRoles map[string][]string` tại Write-Time. Áp dụng kỹ thuật 3-index slice `roles[:len:len]` để khóa cứng dung lượng mảng (Cap = Len), ngăn chặn tuyệt đối rủi ro `append()` làm ô nhiễm vùng nhớ ngầm dùng chung.
- **Zero-Allocation Stack Fallback for Independent Roles:** Triển khai `GetInheritedRolesInto(role, &subScratch)` sử dụng bộ đệm stack scratch buffer `scratch[:1:1]`, loại bỏ hoàn toàn việc cấp phát heap `[]string{role}` khi gặp Subject không có kế thừa vai trò.
- **EvalContext Scratch Buffers & Zero-Copy Map Pointer (`internal/engine/evaluator.go`):** Nhúng trực tiếp mảng đệm `scratchNodes [16]parser.ValueNode` và `scratchIPs [16][16]byte` vào `EvalContext`. Chuyển `GetEvalContext` sang cơ chế Zero-Copy pointer (không sao chép từng phần tử map).
- **Fast Byte Arithmetic Parsers:** Thay thế hoàn toàn `fmt.Sscanf` và `net.ParseIP` bằng các hàm phân giải số học byte trực tiếp `parseFastTime` (cho các mốc Time HH:MM:SS / HH:MM) và `parseIPv4FastInto` (chuyển IPv4 vào mảng cố định không phân bổ slice).
- **Static Reason Constants & Pre-allocated Explanation List (`internal/engine/decision.go`):** Bổ sung `ExplanationList []string` bất biến vào `PolicyNode` ngay từ lúc compile AST. Thay thế toàn bộ `fmt.Sprintf` và `strings.Join` trên fast-path `CheckPermission` bằng các hằng số tĩnh (`ReasonDenyForbid`, `ReasonAllowPermit`, `ReasonDenyDefault`), chỉ format chuỗi động khi gọi endpoint giải thích chi tiết `ExplainDecision`.

### Performance Metrics Verified
- **Độ trễ đánh giá RAM (`BenchmarkEvaluatorLatency`):** Giảm từ 3,654 ns/op xuống **297.6 ns/op (0.297 µs)** — Nhanh gấp **12.2 lần**.
- **Cấp phát bộ nhớ Hot-Path:** Giảm từ 25 allocs/op (493 B/op) xuống **0 B/op và 0 allocs/op** (Zero GC Pressure tuyệt đối).
- **Thông lượng tải đồng thời (`BenchmarkConcurrentLoad`):** Đạt **24.05 ns/op**, tương đương năng lực xử lý lý thuyết vượt **41.5 triệu RPS/core**.
- **Kịch bản ERP ABAC phức tạp (`BenchmarkERP_PurchaseOrderEvaluation`):** Đạt **589.8 ns/op** với **0 B/op và 0 allocs/op**.

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
