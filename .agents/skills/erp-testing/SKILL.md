---
name: erp-testing
description: Expert rules and scenarios for verifying ERP ABAC access control, micro-benchmarks, and performance budgets.
---

# ERP ABAC Testing & Benchmarking Skill

## 🎯 Mission
Validate real-world ERP authorization scenarios, concurrency safety, and sub-millisecond latency budgets.

## 🔑 Critical Implementation Rules
1. **Core ERP ABAC Test Scenarios**:
   - **PO Approval Limits**: Test multi-tier limits (`Manager <= 50M`, `Director <= 500M`, `CFO = any`) + department matching (`principal.dept == resource.dept`).
   - **Separation of Duties (SoD)**: Enforce `forbid(...) when { principal.id == resource.creator_id };`. Creator can NEVER approve own voucher.
   - **Multi-Branch Isolation**: Test `principal.branch == resource.branch || principal.is_group_auditor == "true"`.
   - **Payroll Shielding**: Test time bounds (`08:00:00Z` to `18:00:00Z`), internal IP CIDR (`10.10.0.0/16`), and device/wifi forbid rules.
   - **Role Hierarchy DAG**: Verify transitive closure across organizational layers (`CEO → VP → Director → Manager → Staff`).
2. **Benchmark Commands & Budgets**:
   - Latency Benchmark: `go test -bench=BenchmarkEvaluatorLatency -benchmem ./tests/...` (Target $< 0.35$ms/op, $< 3.5$µs in RAM).
   - Throughput Benchmark: `go test -bench=BenchmarkConcurrentLoad -benchmem ./tests/...` (Target $> 1,000,000$ RPS).
   - Unit & Race Tests: `go test -v -race ./internal/engine ./internal/parser ./tests/...`.

## 📂 Source Files
- [`tests/erp_scenarios_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/erp_scenarios_test.go)
- [`tests/benchmark_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/benchmark_test.go)
- [`tests/e2e_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_test.go)
- [`docs/domain/erp-abac-guide.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/domain/erp-abac-guide.md)
