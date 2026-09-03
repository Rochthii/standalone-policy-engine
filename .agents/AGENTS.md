# Standalone Policy Engine (PDP) — Master Context Guide

> **AI Directive**: This file is the **single source of truth** for project context, invariants, and architecture. Read **ONLY** this file for general tasks. Do **NOT** scan whole folders or pre-load skill files unless specifically implementing deep changes in those subsystems.

> **Author**: Chăm Rốch Thi  
> **Institution**: Posts and Telecommunications Institute of Technology (PTIT)  
> **Core Mission**: Ultra-high-performance, In-Memory Policy Decision Point (PDP) in Go for Cloud-Native microservices, SaaS, and ERP AI Agent Delegation (Odoo 17).  
> **Targets**: Latency < 0.35ms (in-memory < 3.5µs, verified 540ns), Throughput > 1,000,000 RPS, Zero GC allocations on evaluation hot path.  
> **Decision Model**: Deny-by-default, Forbid-overrides, Explicit permit.  

---

## Architecture & Core Data Flow

```
Client / AI Agent (Odoo 17 PEP)
     │
     ▼ (gRPC :50051 CheckAccess / ExplainDecision / RevokeDelegation)
PDP Server (Data Plane)
     │
     ├──► [Layer 1 Security Interceptor]
     │    ├── Anti-TOCTOU: In-Memory RevocationMap O(1) sync.Map (< 50ns)
     │    ├── Proof Verification: HMAC-SHA256 Canonical String + TTL Check
     │    └── Tenant Isolation: claims["tenant_id"] == req.TenantId
     │
     ├──► [Layer 2 In-Memory Evaluation Engine]
     │    ├── In-Memory Trie O(log N) (FNV-1a 64-bit uint64 index)
     │    ├── Role DAG Closure O(1) (Pre-computed Transitive Graph)
     │    └── AST Evaluator (Zero-Alloc, sync.Pool, Bitmask IP, int64 Unix time)
     │         └── Operator `contains` for comma-separated SoD checks
     │
     ▼
Decision: ALLOW / DENY (+ Pre-compiled Obligations: REQUIRE_HUMAN_APPROVAL)
     │
     ├──► Non-Rollback PEP State Machine in Odoo (state -> 'to approve', no DB rollback)
     │
     ▼
Async Ring Buffer Logger ──► Postgres (pgx.CopyFrom) + AES-GCM Encrypt
                             (Fallback: Spill-to-Disk ./spill-logs)
```

### Policy Sync & Distributed Resilience
1. **Cloud-Native 100% Stateless Profile** (`STORAGE_MODE=cloud`): Boots in < 50ms with Zero-Wait Lazy Loading.
2. **Pure PostgreSQL Sequence (Zero Redis)**: Atomic sequence in `tenants.revision`. Emits `NOTIFY policy_events` (< 120B).
3. **Monotonic Gap Detection & Catch-Up**: If `event.Revision > current + 1`, triggers instant Catch-Up sync (< 50ms) instead of waiting for polling.
4. **Edge Embedded Snapshot** (`STORAGE_MODE=edge`): BadgerDB embedded KV store for air-gapped IoT gateways.

---

## Codebase & Component Map

| Component | Path | Description |
|---|---|---|
| **PDP Data Plane** | `cmd/pdp-server/` | gRPC service (:50051) serving `CheckAccess`, `ExplainDecision`, `RevokeDelegation`. |
| **Control Plane** | `cmd/control-plane/` | REST API (:8080) for policy CRUD, simulation (`/simulate`), and Prometheus `/metrics`. |
| **CLI Tool (`pectl`)** | `cmd/pectl/`, `internal/pectl/` | Enterprise CLI for policy management, dry-run simulations, and health checks. |
| **Trie Index** | `internal/engine/trie.go` | Multi-level FNV-1a uint64 index (`Tenant -> Subject -> Resource -> Action`). Slice pooling. |
| **Role DAG** | `internal/engine/dag.go` | DFS cycle detection; pre-computed transitive closure for O(1) `IsDescendant()`. |
| **AST Evaluator** | `internal/engine/evaluator.go` | Zero-alloc evaluator with `sync.Pool`, sentinels (`boolTrue`/`boolFalse`), `contains` SoD. |
| **Engine State** | `internal/engine/engine.go` | Copy-On-Write (COW) lock-free read path via `atomic.LoadPointer` / `StorePointer`. |
| **DSL Compiler** | `internal/parser/` | Lexer & Pratt parser for Cedar-like DSL (`permit`/`forbid`). Max AST depth <= 15, constant folding. |
| **Security & Delegation** | `internal/security/` | JWT tenant isolation, AES-GCM envelope encryption, `delegation.go` (HMAC & RevocationMap O(1)). |
| **Storage & Sync** | `internal/storage/`, `internal/engine/sync.go` | PostgreSQL (`pgx`), BadgerDB edge cache, Postgres `LISTEN/NOTIFY` Fast Gap Catch-Up. |
| **Audit Logger** | `internal/audit/logger.go` | Lock-free ring buffer channel -> `pgx.CopyFrom` batch insert, Spill-to-Disk on failure (1GB cap). |
| **Protobuf Contract** | `proto/v1/policy.proto` | IDL defining `CheckAccess`, `ExplainDecision`, `RevokeDelegation` with custom `"json"` codec. |
| **Seed Policies** | `configs/policies.cedar` | 6 standard P2P ruleset with SoD `contains` operator (`delegation_chain contains creator_id`). |
| **Odoo 17 PEP Addon** | `custom_addons/pdp_authorizer/` | PID-Safe gRPC client, model `pdp.delegation.grant`, non-rollback PEP `purchase.order`. |
| **Frozen Testbed** | `docker-compose.testbed.yml` | Pinned base images (`golang:alpine`, `odoo:17.0`, `postgres:15-alpine`) for 2026–2029. |
| **Tests & Benchmarks**| `tests/` | 7 E2E vectors (`e2e_delegation_test.go`), Baseline benchmark (`baseline_odoo_orm_benchmark.py`). |

---

## Specialized Skills Index (Read On-Demand Only)

Read these skill guides **ONLY** when actively modifying their specific subsystems (< 40 lines each):
- **`agent-authorization`** (`.agents/skills/agent-authorization/SKILL.md`): Canonical String HMAC, In-Memory RevocationMap O(1), Non-Rollback PEP.
- **`grpc-dataplane`** (`.agents/skills/grpc-dataplane/SKILL.md`): gRPC server, JSON codec, RevokeDelegation, Layer 1 interceptor, PID-Safe client.
- **`erp-testing`** (`.agents/skills/erp-testing/SKILL.md`): 7 Delegation vectors, Odoo baseline benchmark (44,000x), 540ns budget.
- **`docker-standards`** (`.agents/skills/docker-standards/SKILL.md`): Pinned Docker testbed (2026–2029), UTF-8 BOM prevention, testbed runner.
- **`dsl-compiler`** (`.agents/skills/dsl-compiler/SKILL.md`): Pratt parser, AST depth limit, `contains` SoD operator, constant folding.
- **`engine-evaluator`** (`.agents/skills/engine-evaluator/SKILL.md`): Trie, DAG Transitive Closure, AST pool, zero-alloc hot path.
- **`storage-audit`** (`.agents/skills/storage-audit/SKILL.md`): `pgx.CopyFrom`, Spill-to-Disk, Postgres LISTEN/NOTIFY sync, BadgerDB.
- **`clean-architecture-standards`** (`.agents/skills/clean-architecture-standards/SKILL.md`): File size budgets ($\le 250$ lines), zero hardcoding, centralized config.
- **`critical-advisor`** (`.agents/skills/critical-advisor/SKILL.md`): Technical stress-testing, blindspot detection, zero sycophancy.

---

## Absolute Rules & Engineering Invariants (Never Violate)

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

## Essential Commands Cheat Sheet

```bash
# 1. Run 7 E2E Delegation Vectors Verification (All 7 PASS)
go test -v ./tests -run=TestE2E_P2P_Delegation_7Vectors

# 2. Run Evaluator Latency & Zero-Allocation Benchmark (540ns, 0 B/op)
go test -bench=BenchmarkEvaluatorLatency -benchmem ./tests/... -run=^$

# 3. Run Core Engine & Parser Unit Tests (with Race Detector)
go test -v -race ./internal/security ./internal/server

# 4. Run Frozen Docker Testbed (Single Command 2026-2029)
docker compose -f docker-compose.testbed.yml up --abort-on-container-exit

# 5. Run Baseline Odoo ORM Benchmark (44,000x speedup comparison)
python tests/baseline_odoo_orm_benchmark.py
```