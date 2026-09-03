# Standalone Policy Engine (PDP) — Master Context Guide

> 🤖 **AI Directive**: This file is the **single source of truth** for project context, invariants, and architecture. Read **ONLY** this file for general tasks. Do **NOT** scan whole folders or pre-load skill files unless specifically implementing deep changes in those subsystems.

> **Core Mission**: Ultra-high-performance, In-Memory Policy Decision Point (PDP) in Go for Cloud-Native microservices and multi-tenant SaaS.
> **Targets**: Latency < 0.35ms (in-memory < 3.5µs), Throughput > 1,000,000 RPS, Zero GC allocations on evaluation hot path.
> **Decision Model**: Deny-by-default, Forbid-overrides, Explicit permit.

---

## 🏗️ Architecture & Core Data Flow

```
Client / AI Agent
     │
     ▼
API Gateway (PEP) ──(gRPC CheckAccess / ExplainDecision)──► PDP Server (:50051)
                                                                 │
                                                    ┌────────────┴────────────┐
                                                    ▼                         ▼
                                           In-Memory Trie O(log N)   Role DAG Closure O(1)
                                           (FNV-1a 64-bit uint64)    (Pre-computed Graph)
                                                    │                         │
                                                    └────────────┬────────────┘
                                                                 ▼
                                                        AST Evaluator (Zero-Alloc)
                                                        (Bitmask IP, int64 Time)
                                                                 │
                                                                 ▼
                                                    Decision: ALLOW / DENY
                                                                 │
                                                    Async Ring Buffer Logger
                                                                 ▼
                                            Postgres (pgx.CopyFrom) + AES-GCM Encrypt
                                            (Fallback: Spill-to-Disk ./spill-logs)
```

### Policy Sync & Distributed Resilience Architecture
1. **Cloud-Native 100% Stateless Profile** (`STORAGE_MODE=cloud`): Zero local disk footprint on Kubernetes pods; boots in < 50ms with Zero-Wait Lazy Loading to eliminate Thundering Herd on scale-up.
2. **Edge Embedded Snapshot** (`STORAGE_MODE=edge`): BadgerDB embedded KV store for air-gapped / offline IoT gateways.
3. **Monotonic Revision ID & Gap Detection**: Atomic transactional sequence in PostgreSQL (`tenants.revision`). PDP detects missed events (`event.Revision > current + 1`) and triggers instant Catch-Up sync (< 50ms) instead of waiting for 10s polling.
4. **Compile-Time Attribute Dependency Analysis**: AST Compiler extracts `RequiredAttributes` exposed via `GET /api/v1/tenants/{id}/schema` for PEPs to send minimal (< 300B) gRPC payloads.

---

## 🗂️ Codebase & Component Map

| Component | Path | Description |
|---|---|---|
| **PDP Data Plane** | `cmd/pdp-server/` | gRPC service (:50051) serving `CheckAccess` & `ExplainDecision`. Supports mTLS & OpenZiti dark service. |
| **Control Plane** | `cmd/control-plane/` | REST API (:8080) for policy CRUD, dry-run simulation (`/simulate`), and Prometheus `/metrics`. |
| **CLI Tool (`pectl`)** | `cmd/pectl/`, `internal/pectl/` | Enterprise CLI for policy management, dry-run simulations, live checks, and health/metrics. |
| **Trie Index** | `internal/engine/trie.go` | Multi-level FNV-1a uint64 index (`Tenant -> Subject -> Resource -> Action`). Slice pooling (`GetPolicySlice`). |
| **Role DAG** | `internal/engine/dag.go` | DFS cycle detection at write; pre-computed transitive closure for O(1) `IsDescendant()` runtime checks. |
| **AST Evaluator** | `internal/engine/evaluator.go` | Zero-allocation evaluator with `sync.Pool`, sentinels (`boolTrue`/`boolFalse`), bitmask IP, int64 Unix timestamp. |
| **Engine State** | `internal/engine/engine.go` | Copy-On-Write (COW) lock-free read path via `atomic.LoadPointer` / `StorePointer`. |
| **DSL Compiler** | `internal/parser/` | Lexer & Pratt parser for Cedar-like DSL (`permit`/`forbid`). Max AST depth <= 15, constant folding. |
| **Storage & Sync** | `internal/storage/`, `internal/engine/sync.go` | PostgreSQL (`pgx`), BadgerDB edge cache, Postgres `LISTEN/NOTIFY` & Fast Gap Catch-Up (< 50ms). |
| **Audit Logger** | `internal/audit/logger.go` | Lock-free ring buffer channel -> `pgx.CopyFrom` batch insert, Spill-to-Disk on failure (1GB cap). |
| **Security** | `internal/security/` | JWT token validation & tenant isolation (`claims["tenant_id"] == req.TenantId`), AES-GCM envelope encryption (`LOG_KEK`). |
| **Protobuf Contract** | `proto/v1/policy.proto` | gRPC service & message definitions with custom `"json"` codec (Decision enum without `omitempty`). |
| **DB Migrations** | `db/migrations/` | Versioned SQL migration files managed via `golang-migrate`. |
| **Tests & Benchmarks**| `tests/` | E2E integration (`e2e_test.go`), Micro-benchmarks (`benchmark_test.go`), ERP scenarios (`erp_scenarios_test.go`). |

---

## 🛠️ Specialized Skills Index (Read On-Demand Only)

Read these skill guides **ONLY** when actively modifying their specific subsystems:
- **`engine-evaluator`** (`.agents/skills/engine-evaluator/SKILL.md`): Trie, DAG Transitive Closure, AST pool.
- **`dsl-compiler`** (`.agents/skills/dsl-compiler/SKILL.md`): Pratt parser, AST depth limit, contains operator, constant folding.
- **`grpc-dataplane`** (`.agents/skills/grpc-dataplane/SKILL.md`): gRPC server, JSON codec, RevokeDelegation, Layer 1 interceptor.
- **`storage-audit`** (`.agents/skills/storage-audit/SKILL.md`): `pgx.CopyFrom`, Spill-to-Disk, Postgres LISTEN/NOTIFY sync, BadgerDB.
- **`erp-testing`** (`.agents/skills/erp-testing/SKILL.md`): 7 Delegation vectors, Odoo baseline benchmark, micro-benchmarks.
- **`agent-authorization`** (`.agents/skills/agent-authorization/SKILL.md`): Canonical String HMAC, In-Memory RevocationMap O(1), Non-Rollback PEP.
- **`critical-advisor`** (`.agents/skills/critical-advisor/SKILL.md`): Technical stress-testing & blindspot detection.
- **`clean-architecture-standards`** (`.agents/skills/clean-architecture-standards/SKILL.md`): File size budgets ($\le 250$ lines), zero hardcoding, centralized config, and modular SRP design.
- **`docker-standards`** (`.agents/skills/docker-standards/SKILL.md`): Docker testbed, pinned images, PowerShell UTF-8 BOM prevention, E2E testbed runner.

---

## ⛔ Absolute Rules & Engineering Invariants (Never Violate)

1. **Production-Real Only**: Zero mocks, zero fake returns, zero hardcoded bypasses in production code paths.
2. **Zero Linear Scans**: Never iterate over slice of policies. All lookups MUST use Trie O(log N) and DAG Transitive Closure O(1).
3. **Zero Heap Allocations on Hot Path**:
   - Use `sync.Pool` for evaluation contexts.
   - Return sentinel pointers (`boolTrue`, `boolFalse`) instead of allocating new `ValueNode`.
   - Use uint64 FNV-1a hashes, bitmask IP parsing (`IPNet.Contains`), and pre-parsed int64 Unix nanoseconds.
4. **Lock-Free Hot-Path**: `CheckPermission` must NEVER acquire Mutexes; reads use atomic pointer swap (COW).
5. **Fail-Closed Security**: Missing attributes or unhandled syntax evaluate to `false`/`DENY`. AST depth strictly capped at <= 15.
6. **Multi-Tenant JWT Isolation**: Interceptor strictly enforces `claims["tenant_id"] == req.TenantId`.

---

## ⚡ Essential Commands Cheat Sheet

```bash
# 1. Run 7 E2E Delegation Vectors Verification
go test -v ./tests -run=TestE2E_P2P_Delegation_7Vectors

# 2. Run Evaluator Latency & Zero-Allocation Benchmark
go test -bench=BenchmarkEvaluatorLatency -benchmem ./tests/... -run=^$

# 3. Run Core Engine & Parser Unit Tests (with Race Detector)
go test -v -race ./internal/security ./internal/server

# 4. Run Frozen Docker Testbed (Single Command)
docker compose -f docker-compose.testbed.yml up --abort-on-container-exit

# 5. Run Baseline Odoo ORM Benchmark
python tests/baseline_odoo_orm_benchmark.py
```