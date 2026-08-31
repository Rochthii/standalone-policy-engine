# Standalone PDP Engine — AI Master Guide

## 🎯 Core Mission
High-performance In-Memory Policy Decision Point (PDP) in Go.
- **Targets**: Latency $< 0.35$ms, Throughput $> 1,000,000$ RPS, Zero GC pressure.
- **Model**: Deny-by-default, Forbid-overrides, Explicit permit.

---

## ⛔ Absolute Rules (Never Violate)
1. **Production-Real Only**: No mocks, no fake returns, no demo buttons, no hardcoded bypasses.
2. **Zero Linear Scans**: Never loop full policy lists. Use In-Memory Trie $O(\log N)$ and Role DAG Transitive Closure $O(1)$.
3. **Zero Heap Allocations on Hot Path**: Must use `sync.Pool`, stack buffers, sentinel booleans (`boolTrue`/`boolFalse`), and pre-parsed IP/Unix timestamp.
4. **JSON-over-gRPC Codec**: Enforce custom `"json"` codec. `decision` enum must **never** have `omitempty`.
5. **Fail-Closed Security**: Missing attributes evaluate to `false`/`DENY`. Stack depth strictly capped at $\le 15$.

---

## 🗂️ Specialized Sub-Agents / Skills Map
- [**Engine & Evaluator**](file:///e:/Projects/Project_TN/standalone-policy-engine/.agents/skills/engine-evaluator/SKILL.md): Trie Index, Role DAG, COW atomic pointer swap, AST Evaluator.
- [**DSL & Compiler**](file:///e:/Projects/Project_TN/standalone-policy-engine/.agents/skills/dsl-compiler/SKILL.md): Lexer, Pratt Parser, Type Checking, Constant Folding, AST depth limit.
- [**gRPC Data Plane**](file:///e:/Projects/Project_TN/standalone-policy-engine/.agents/skills/grpc-dataplane/SKILL.md): `CheckAccess`, `ExplainDecision`, mTLS, Keep-Alive, OpenZiti Dark Service.
- [**Storage & Audit**](file:///e:/Projects/Project_TN/standalone-policy-engine/.agents/skills/storage-audit/SKILL.md): PostgreSQL `pgx.CopyFrom`, Ring Buffer, Spill-to-Disk, Redis Pub/Sub, BadgerDB.
- [**ERP Testing & Benchmark**](file:///e:/Projects/Project_TN/standalone-policy-engine/.agents/skills/erp-testing/SKILL.md): ERP ABAC scenarios (PO limits, SoD, Multi-Branch, Payroll), Micro-benchmarks.
- [**AI Agent Authorization**](file:///e:/Projects/Project_TN/standalone-policy-engine/.agents/skills/agent-authorization/SKILL.md): Unified Subject Model, Delegation Chains, Tool-Call Contexts, Tri-State Decisions (`ALLOW`/`DENY`/`REQUIRE_HUMAN_APPROVAL`), Deterministic Guardrails (NIST AI RMF / OWASP LLM06).

---

## 📍 Key File Router
| File | Role |
|---|---|
| [`cmd/pdp-server/main.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go) | Data Plane gRPC Server Entry |
| [`internal/server/grpc_server.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/grpc_server.go) | gRPC Handler, JWT Isolation & Trace |
| [`internal/engine/trie.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go) | FNV-1a Hashed Trie Index |
| [`internal/engine/evaluator.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go) | Zero-Allocation AST Evaluator |
| [`internal/engine/dag.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go) | Role DAG Transitive Closure |
| [`internal/audit/logger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go) | Ring Buffer & Spill-to-Disk Logger |
| [`proto/v1/policy.proto`](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto) | Protobuf & gRPC Interface Definition |
