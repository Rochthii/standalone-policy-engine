---
name: erp-testing
description: Expert rules and scenarios for ERP ABAC access control, 7 Test Vectors, and Baseline benchmarks.
---

# ERP ABAC Testing & Benchmarking Skill

## 🎯 Mission
Validate real-world ERP authorization scenarios, 7 delegation test vectors, and sub-microsecond latency budgets.

## 🔑 Critical Implementation Rules
1. **7 E2E Delegation Test Vectors ([`tests/e2e_delegation_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_delegation_test.go))**:
   - `TC-01`: Manager creates PO and self-approves $\to$ `DENY` (SoD Collision).
   - `TC-02`: AI Agent approves PO created by Delegator $\to$ `DENY` (SoD Chain).
   - `TC-03`: Agent autonomous PO $\le \$2,000 \to$ `ALLOW`.
   - `TC-04`: Agent PO $> \$2,000 \to$ `DENY` (`REQUIRE_HUMAN_APPROVAL`).
   - `TC-05`: Tampered HMAC amount ($50,000) $\to$ `403 PermissionDenied`.
   - `TC-06`: Revoked Grant on RAM $\to$ `DENY` (`POL-REVOCATION-BLACK-LIST`).
   - `TC-07`: Expired TTL Proof $\to$ `403 PermissionDenied`.

2. **Benchmark Commands & Budgets**:
   - Evaluator Latency: `go test -bench=BenchmarkEvaluatorLatency -benchmem ./tests/... -run=^$`
     - Verified result: **$540.2\ \text{ns/op}$, `0 B/op`, `0 allocs/op`** (Budget: $< 3.5\,\mu\text{s}$).
   - 7 Vectors Test: `go test -v ./tests -run=TestE2E_P2P_Delegation_7Vectors` (All 7 pass in $\sim 4.5$s).
   - Race Tests: `go test -v -race ./internal/security ./internal/server`.

3. **Baseline Comparison vs Odoo Native ORM ([`tests/baseline_odoo_orm_benchmark.py`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/baseline_odoo_orm_benchmark.py))**:
   - Odoo Native `ir.rule` + Postgres: **$23.77\ \text{ms}$**, ~24 KB heap allocs.
   - Standalone Go PDP In-Memory: **$0.00054\ \text{ms}$ ($540\ \text{ns}$)**, 0 B/op.
   - Go PDP is **$\approx 44,000$x faster**, eliminates TOCTOU, and avoids Odoo Rollback Trap.

## 📂 Source Files
- [`tests/e2e_delegation_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_delegation_test.go)
- [`tests/baseline_odoo_orm_benchmark.py`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/baseline_odoo_orm_benchmark.py)
- [`docs/technical-spec/EVALUATION_MATRIX.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/EVALUATION_MATRIX.md)
- [`docs/technical-spec/BENCHMARK_REPRODUCIBILITY.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/BENCHMARK_REPRODUCIBILITY.md)
