# Sprint 7 – Detailed Task Breakdown

## Critical (Must‑fix before release)

### 1️⃣ Tenant Isolation – JWT `tenant_id` validation
- **Goal**: Ensure the `tenant_id` in the request matches the one embedded in the JWT token for both gRPC and HTTP endpoints.
- **Sub‑tasks**:
  1. Create a **gRPC interceptor** `TenantValidator` that extracts JWT from metadata, validates the token (reuse `internal/security/jwt.go`), extracts `tenant_id` claim and compares it with `req.TenantId`.
  2. Return `PERMISSION_DENIED` on mismatch.
  3. Add **HTTP middleware** `TenantAuthMiddleware` that reads the `Authorization` header (or cookie), validates the JWT, stores `tenant_id` in request context, and checks the path parameter against the stored value.
  4. Write unit tests:
     - `grpc_server_test.go` – simulate a request with mismatched tenant ids.
     - `http_server_test.go` – same scenario for the REST API.
  5. Update CI pipeline to run the new tests.
- **Estimated effort**: 2–3 h.
- **Files to modify**: `internal/server/grpc_server.go`, `internal/server/http_server.go`, new test files, possibly `internal/server/middleware.go` for shared logic.

### 2️⃣ Fix import order in `sync.go`
- **Goal**: Move `import "sync"` to the top of the import block.
- **Sub‑tasks**:
  1. Edit `internal/engine/sync.go` – relocate import statement.
  2. Run `go vet` / `golangci-lint` to confirm no style warnings.
- **Estimated effort**: < 5 min.

### 3️⃣ Add context timeout to gRPC handlers
- **Goal**: Prevent long‑running requests from blocking the gRPC worker pool.
- **Sub‑tasks**:
  1. Wrap the body of `CheckAccess` in `context.WithTimeout(ctx, 100*time.Millisecond)`.
  2. Propagate the derived context to `engine.CheckPermission` (adjust signature if necessary).
  3. Return `DEADLINE_EXCEEDED` when the timeout expires.
  4. Add a benchmark to verify latency impact.
- **Estimated effort**: 1 h.
- **Files to modify**: `internal/server/grpc_server.go`, possibly `internal/engine/engine.go`.

---

## High Priority (1‑2 weeks)

### 4️⃣ E2E Integration Test (Docker‑Compose)
- **Goal**: Verify the full control‑plane → PDP → decision flow.
- **Sub‑tasks**:
  1. Add `tests/docker-compose.yml` with services: PostgreSQL, Redis, `pdp-server`, `control-plane`.
  2. Write `tests/e2e_test.go` that:
     - Creates a tenant, posts a draft policy, publishes it, then calls the gRPC `CheckAccess` endpoint.
     - Checks that the decision matches expected (ALLOW/DENY).
     - Simulates policy update and verifies immediate decision change via Pub/Sub.
     - Simulates Redis outage and ensures fallback polling works after 10 s.
  3. Add fixtures (`tests/fixtures/policies.json`).
  4. Integrate the test into CI (run only on PRs targeting `main`).
- **Estimated effort**: 3‑4 h.

### 5️⃣ Database Migration System
- **Goal**: Replace inline DDL with versioned migrations using `golang‑migrate`.
- **Sub‑tasks**:
  1. Install migration tool (`go get -u github.com/golang-migrate/migrate/v4/cmd/migrate`).
  2. Create `db/migrations/` with:
     - `000001_init_schema.up.sql` / `.down.sql` (current schema).
     - `000002_add_audit_logs_table.up.sql` / `.down.sql` (if not already present).
  3. Remove `initSchema()` implementation from `internal/storage/postgres.go`.
  4. Add migration runner in `postgres.go` (use `migrate.New("file://db/migrations", dsn)`).
  5. Write a small integration test that runs migrations against an in‑memory PostgreSQL container.
- **Estimated effort**: 2‑3 h.

### 6️⃣ Benchmark Validation with Real Data
- **Goal**: Produce concrete latency numbers for 1k, 10k, 100k policies and concurrent load.
- **Sub‑tasks**:
  1. Add benchmark files (`tests/benchmark_test.go`) with:
     - `BenchmarkEvaluatorLatency`
     - `BenchmarkConcurrentLoad`
  2. Run benchmarks for each scale, capture CPU/memory profiles (`-cpuprofile`, `-memprofile`).
  3. Summarize results in `benchmarks/results.md`.
  4. If latency exceeds 1 ms at 100k policies, create a follow‑up ticket for Trie/LRU optimisation.
- **Estimated effort**: 2‑3 h.

---

## Medium Priority (1 month)

### 7️⃣ Tenant Auth Integration (JWT parsing middleware)
- Create `internal/auth/jwt.go` with `ExtractTenantFromJWT(tokenString, secret) (string, error)`.
- Create `internal/server/middleware.go` exposing `AuthMiddleware(secret string) func(http.Handler) http.Handler`.
- Wire middleware in `http_server.go` and gRPC interceptor chain.
- Add unit tests `internal/auth/jwt_test.go`.
- **Effort**: 2‑3 h.

### 8️⃣ Graceful Degradation – Back‑off & Spill Size Limit
- Extend `AuditLogger.replayWorker` with exponential back‑off (initial 1 s, max 1 min).
- Add directory‑size check (`l.getDirSize()`) and rotation when > 1 GB.
- Write tests for back‑off reset and size‑limit logic.
- **Effort**: 1‑2 h.

### 9️⃣ GC Configuration via Environment Variables
- Define `GCConfig` struct (enabled, idle timeout, interval, max cache size).
- Update `NewEngineWithGC(cfg GCConfig)` to respect the config.
- Parse env vars in `cmd/pdp-server/main.go` (`GC_IDLE_TIMEOUT`, `GC_INTERVAL`, `DISABLE_GC`).
- Document variables in `README.md`.
- **Effort**: 1 h.

---

## Low / Nice‑to‑have (post‑2029)

- **Policy Language Extensions** – regex, integer ranges, custom functions.
- **Observability Enhancements** – OpenTelemetry instrumentation, Grafana dashboards, alert rules.

---

## How to Use This Task File
- Save this file as `tasks/sprint7_tasks.md` in the repository root.
- Each top‑level item can be turned into a GitHub issue or project card.
- The **Estimated effort** column is a rough guide for sprint planning.
- When a sub‑task is completed, mark it with `[x]`.

---

*Prepared for the Standalone Policy Engine project*
