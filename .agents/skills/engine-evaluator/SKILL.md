---
name: engine-evaluator
description: Expert rules and constraints for optimizing In-Memory Trie, Role DAG, Copy-On-Write, and AST Evaluator.
---

# In-Memory Engine & Evaluator Skill

## 🎯 Mission
Maintain and optimize sub-microsecond in-memory policy lookup and AST expression evaluation with strict Zero-Allocation ($0$ B/op, $0$ allocs/op) across ultra-extreme DAG and 10,000+ policy workloads.

## 🔑 Critical Implementation Rules
1. **Trie Matching ($O(\log N)$)**:
   - Structure: `TenantID → Subject (uint64) → Resource (uint64) → Action (uint64)`.
   - Keys: FNV-1a 64-bit hash (`fnvHash(s) uint64`), never raw string map keys on hot path.
   - Global Wildcard Partition: Store `principal == any && resource == any` in `GlobalPolicies []*PolicyNode`.
   - Slice Pooling: Always use `GetPolicySlice()` & `ReturnPolicySlice()` via `LookupPoliciesInto` (initial pool capacity $\ge 32$).
   - Stack Scratch Buffers: `ensureAnyInto` and `LookupPoliciesInto` use `[32]string` stack scratch to prevent slice grow / heap escape on deep DAG lookups.
2. **Role Hierarchy DAG ($O(1)$)**:
   - Write path: Cycle detection via DFS. Pre-compute Transitive Closure in `closure map[string]map[string]bool`.
   - Read path: `IsDescendant()` must be instant $O(1)$ lookup.
   - Pre-computed Inherited Roles: `g.inheritedRoles[role]` returned directly (0 allocation), with `scratch *[16]string` fallback for non-DAG subjects.
3. **Copy-On-Write (COW)**:
   - State swaps: Atomically replace `*EngineState` via `atomic.StorePointer` & `atomic.LoadPointer`.
   - Reads are $100\%$ lock-free. Never acquire mutexes on `CheckPermission` path.
4. **AST Evaluator**:
   - Reuse: `GetEvalContext()` from `contextPool` and `Release()` back.
   - Stack Buffers: `scratchNodes [64]parser.ValueNode` and `scratchIPs [64][16]byte` inside `EvalContext` support up to 64 AST nodes without heap allocations.
   - Sentinels: Return read-only `boolTrue` / `boolFalse` pointers instead of allocating new `*ValueNode`.
   - Short-circuit: Immediately return on `false` for `&&` or `true` for `||`.
   - Pre-parsed types: Bitwise `IPNet.Contains(IP)` and `int64` integer comparison for DateTime/Time.
   - Context lookup: Support both prefixed (`principal.field`, `resource.field`, `context.field`) and raw `field`.

## 📊 Benchmark Verification Targets
- Single Evaluation Latency: $< 320$ ns/op ($0$ B/op, $0$ allocs/op).
- Concurrent Throughput: $> 35,000,000$ RPS on 20 cores (`BenchmarkConcurrentLoad`: 27.12 ns/op).
- Deep 11-level DAG + 5,000 policies + 10 ABAC conditions: $< 850$ ns/op (`BenchmarkUltraExtreme_DeepDAG_HeavyABAC`: 810.9 ns/op, $0$ B/op, $0$ allocs/op).
- 10,000 Policies Load: $< 40$ ns/op (`BenchmarkUltraExtreme_10kPolicies_ConcurrentContention`: 35.94 ns/op, $0$ B/op, $0$ allocs/op).

## 📂 Source Files
- [`internal/engine/trie.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go)
- [`internal/engine/evaluator.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go)
- [`internal/engine/dag.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go)
- [`internal/engine/decision.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/decision.go)
- [`internal/engine/engine.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine.go)
- [`internal/engine/engine_gc.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_gc.go)
- [`tests/benchmark_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/benchmark_test.go)
