---
name: engine-evaluator
description: Expert rules for optimizing In-Memory Trie, Role DAG, Copy-On-Write, and AST Evaluator.
---

# In-Memory Engine & Evaluator Skill

## 🎯 Mission
Maintain sub-microsecond in-memory policy evaluation with strict Zero-Allocation ($0$ B/op, $0$ allocs/op) on the evaluation hot-path.

## 🔑 Critical Implementation Rules
1. **Trie Matching ($O(\log N)$)**:
   - Index: `TenantID → Subject (uint64) → Resource (uint64) → Action (uint64)`.
   - Keys: FNV-1a 64-bit hash (`fnvHash(s) uint64`), never raw string maps on hot path.
   - Slice Pooling: Always use `GetPolicySlice()` & `ReturnPolicySlice()` via `LookupPoliciesInto`.
   - Global Wildcards: Partition `principal == any && resource == any` into `GlobalPolicies`.

2. **Role Hierarchy DAG ($O(1)$)**:
   - Write path: DFS cycle detection; pre-compute transitive closure in `closure map[string]map[string]bool`.
   - Read path: `IsDescendant()` is instant $O(1)$ lookup. Pre-computed inherited roles returned with 0 allocation.

3. **Copy-On-Write (COW)**:
   - Atomic pointer swap via `atomic.StorePointer` / `LoadPointer`.
   - Reads are $100\%$ lock-free. `CheckPermission` NEVER acquires mutexes.

4. **AST Evaluator & Zero Allocations**:
   - Reuse `EvalContext` via `sync.Pool`.
   - Sentinels: Return pre-allocated `boolTrue` / `boolFalse` pointers instead of allocating new `*ValueNode`.
   - Pre-parsed types: Bitmask `IPNet.Contains(IP)` and `int64` integer comparison for DateTime/Time.
   - Comma-separated string check: `contains` operator splits by comma for SoD checks.

## 📊 Performance Budget
- Evaluator Latency: **$\approx 540\ \text{ns/op}$** ($0$ B/op, $0$ allocs/op) verified by `BenchmarkEvaluatorLatency`.
- Throughput: $> 1,000,000$ RPS concurrent evaluation.

## 📂 Source Files
- [`internal/engine/trie.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go)
- [`internal/engine/evaluator.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go)
- [`internal/engine/dag.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go)
- [`internal/engine/engine.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine.go)
