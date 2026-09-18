# Real Odoo/PostgreSQL Purchase-Confirmation Evidence — 2026-09-18

## Verdict

`PERF-ODOO-03` replaces the retired sleep/hardcoded comparison with a reproducible, warm-path purchase-confirmation measurement on commit `64494d9`.

For this one business-transaction workload, the PDP PEP path has a higher mean (58.076 ms vs 29.742 ms), p50 (50.523 ms vs 24.906 ms) and p99 (139.146 ms vs 76.405 ms) than native Odoo. This is not a universal Odoo, ERP throughput, concurrent-load or production-performance claim.

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
| Mean | 29.742050 ms | 58.075744 ms |
| p50 | 24.906115 ms | 50.523270 ms |
| p95 | 57.740561 ms | 88.694339 ms |
| p99 | 76.404948 ms | 139.145658 ms |
| Mean-derived throughput | 33.62 ops/s | 17.22 ops/s |

## Reproduce

Run `make benchmark-odoo-orm` from the repository root. It creates an isolated `odoo_orm_benchmark` database, generates ephemeral test certificates, and writes the raw output below. The 2026-09-18 evidence run used the equivalent PowerShell commands with `PDP_GIT_COMMIT=64494d9` and the benchmark Compose profile.

Recorded environment: commit `64494d9`; 13th Gen Intel(R) Core(TM) i7-13700H; Odoo 17 testbed; PostgreSQL 15 testbed; Go `go1.25.14 linux/amd64` from the pinned `golang:1.25-alpine@sha256:1ae0735f...` build image; Python 3.10.12; Docker on `Linux-6.6.87.2-microsoft-standard-WSL2-x86_64-with-glibc2.35`. Each path has 3 samples × 250 requests = 750 raw observations.

## Limits

- This is one warm purchase-confirmation scenario, not inventory reservation, a full purchase workflow, a committed database transaction or a concurrent-load test.
- The final database commit is deliberately excluded; the Odoo test harness rolls the enclosing test transaction back.
- Use the raw distributions, not the mean-derived throughput, for tail-latency conclusions.

## Raw evidence

[`ODOO_ORM_COMPARISON_2026_09_15.json`](./ODOO_ORM_COMPARISON_2026_09_15.json) contains all 1,500 latency observations, method metadata, platform, and commit. The Odoo test completed with `0 failed, 0 error(s)`.
