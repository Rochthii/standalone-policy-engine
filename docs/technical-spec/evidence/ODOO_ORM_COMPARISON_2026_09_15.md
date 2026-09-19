# Real Odoo/PostgreSQL Purchase-Confirmation Evidence — 2026-09-19

## Verdict

`PERF-ODOO-03` replaces the retired sleep/hardcoded comparison with a reproducible, warm-path purchase-confirmation measurement on commit `e243db5`.

For this one business-transaction workload, the PDP PEP path has a higher mean (71.525 ms vs 29.933 ms), p50 (66.719 ms vs 28.641 ms) and p99 (101.640 ms vs 48.843 ms) than native Odoo. This is not a universal Odoo, ERP throughput, concurrent-load or production-performance claim.

## Equal workload and method

- Each request creates and confirms one low-value `purchase.order` through the real Odoo business method.
- Native path: Odoo `purchase.order.button_confirm` backed by PostgreSQL.
- PDP path: the repository-owned PEP `button_confirm` invokes mTLS gRPC with JWT and a full-tuple delegation proof, then calls Odoo's base confirmation method on `ALLOW`.
- Setup, policy/database seeding, and one warm-up per path are excluded. The purchase-confirmation mutation is timed; the final database commit is excluded because Odoo's test harness rolls the test transaction back.
- Each path records 3 samples of 250 per-request `perf_counter_ns` values (750 measurements per path).

The source test is `custom_addons/pdp_authorizer/tests/test_odoo_orm_benchmark.py`; the frozen testbed runner is `deployments/docker/run-odoo-orm-benchmark.py`.

## Result

| Metric | Native Odoo + PostgreSQL | Odoo PDP PEP + mTLS gRPC |
|---|---:|---:|
| Mean | 29.933256 ms | 71.524813 ms |
| p50 | 28.641220 ms | 66.718540 ms |
| p95 | 40.807260 ms | 87.975326 ms |
| p99 | 48.843131 ms | 101.640277 ms |
| Mean-derived throughput | 33.41 ops/s | 13.98 ops/s |

## Reproduce

Run `make benchmark-odoo-orm` from the repository root. It creates an isolated `odoo_orm_benchmark` database, generates ephemeral test certificates, and writes the raw output below. The 2026-09-19 evidence run used the equivalent PowerShell commands with `PDP_GIT_COMMIT=e243db5` and the benchmark Compose profile.

Recorded environment: commit `e243db5`; Odoo 17 testbed; PostgreSQL 15 testbed; Go `go1.25.14 linux/amd64` from the pinned `golang:1.25-alpine@sha256:1ae0735f...` build image; Python 3.10.12; Docker Engine 29.1.3 on `Linux-6.6.87.2-microsoft-standard-WSL2-x86_64-with-glibc2.35`. Each path has 3 samples × 250 requests = 750 raw observations.

## Limits

- This is one warm purchase-confirmation scenario, not inventory reservation, a full purchase workflow, a committed database transaction or a concurrent-load test.
- The final database commit is deliberately excluded; the Odoo test harness rolls the enclosing test transaction back.
- Use the raw distributions, not the mean-derived throughput, for tail-latency conclusions.
- The PDP testbed uses `APP_ENV=test` with real mTLS and PostgreSQL. It deliberately does not configure an external audit archive; therefore it is not production or WORM evidence.

## Raw evidence

[`ODOO_ORM_COMPARISON_2026_09_15.json`](./ODOO_ORM_COMPARISON_2026_09_15.json) contains all 1,500 latency observations, method metadata, platform, and commit. The Odoo test completed with `0 failed, 0 error(s)`.
