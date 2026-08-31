---
name: engine-evaluator
description: Expert rules and constraints for optimizing In-Memory Trie, Role DAG, Copy-On-Write, and AST Evaluator.
---

# In-Memory Engine & Evaluator Skill

## 🎯 Mission
Maintain and optimize sub-microsecond in-memory policy lookup and AST expression evaluation.

## 🔑 Critical Implementation Rules
1. **Trie Matching ($O(\log N)$)**:
   - Structure: `TenantID → Subject (uint64) → Resource (uint64) → Action (uint64)`.
   - Keys: FNV-1a 64-bit hash (`fnvHash(s) uint64`), never raw string map keys on hot path.
   - Global Wildcard Partition: Store `principal == any && resource == any` in `GlobalPolicies []*PolicyNode`.
   - Slice Pooling: Always use `GetPolicySlice()` & `ReturnPolicySlice()` via `LookupPoliciesInto`.
2. **Role Hierarchy DAG ($O(1)$)**:
   - Write path: Cycle detection via DFS. Pre-compute Transitive Closure in `closure map[string]map[string]bool`.
   - Read path: `IsDescendant()` must be instant $O(1)$ lookup.
3. **Copy-On-Write (COW)**:
   - State swaps: Atomically replace `*EngineState` via `atomic.StorePointer` & `atomic.LoadPointer`.
   - Reads are $100\%$ lock-free. Never acquire mutexes on `CheckPermission` path.
4. **AST Evaluator**:
   - Reuse: `GetEvalContext()` from `contextPool` and `Release()` back.
   - Sentinels: Return read-only `boolTrue` / `boolFalse` pointers instead of allocating new `*ValueNode`.
   - Short-circuit: Immediately return on `false` for `&&` or `true` for `||`.
   - Pre-parsed types: Bitwise `IPNet.Contains(IP)` and `int64` integer comparison for DateTime/Time.
   - Context lookup: Support both prefixed (`principal.field`, `resource.field`, `context.field`) and raw `field`.

## 📂 Source Files
- [`internal/engine/trie.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go)
- [`internal/engine/evaluator.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go)
- [`internal/engine/dag.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go)
- [`internal/engine/decision.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/decision.go)
- [`internal/engine/engine.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine.go)
- [`internal/engine/engine_gc.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_gc.go)
