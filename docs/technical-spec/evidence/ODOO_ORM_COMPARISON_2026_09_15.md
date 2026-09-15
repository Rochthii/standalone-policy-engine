# Real Odoo ORM/PostgreSQL Comparison — 2026-09-15

## Verdict

`PERF-ODOO-03` replaces the retired sleep/hardcoded comparison with a reproducible, warm-path measurement on commit `4bb4c48`.

For this one authorization workload, the PDP path has a lower mean (1.037 ms vs 1.396 ms, 1.346x) and p50 (0.941 ms vs 1.365 ms, 1.450x). Its p99 is higher (2.885 ms vs 2.412 ms). This is not a universal Odoo, ERP transaction, throughput, or production-performance claim.

## Equal workload and method

- One existing low-value `purchase.order` made by a different user is authorized.
- Native path: Odoo ORM `search` guarded by a real `ir.rule`, backed by PostgreSQL.
- PDP path: the Odoo generated Python client performs an mTLS gRPC `CheckAccess` with JWT and a full-tuple delegation proof.
- Setup, policy/database seeding, and one warm-up per path are excluded. No business mutation is timed.
- Each path records 3 samples of 250 per-request `perf_counter_ns` values (750 measurements per path).

The source test is `custom_addons/pdp_authorizer/tests/test_odoo_orm_benchmark.py`; the frozen testbed runner is `deployments/docker/run-odoo-orm-benchmark.py`.

## Result

| Metric | Odoo ORM + PostgreSQL | Odoo client + PDP mTLS gRPC |
|---|---:|---:|
| Mean | 1.395756 ms | 1.036841 ms |
| p50 | 1.364612 ms | 0.941398 ms |
| p95 | 1.873552 ms | 1.807840 ms |
| p99 | 2.412293 ms | 2.884821 ms |
| Mean-derived throughput | 716.46 ops/s | 964.47 ops/s |

## Reproduce

Run `make benchmark-odoo-orm` from the repository root. It creates an isolated `odoo_orm_benchmark` database, generates ephemeral test certificates, and writes the raw output below.

Recorded environment: commit `4bb4c48`; Odoo 17 testbed; PostgreSQL 15 testbed; Go `go1.25.14 linux/amd64` from the pinned `golang:1.25-alpine@sha256:1ae0735f...` build image; Python 3.10.12; Docker on `Linux-6.6.87.2-microsoft-standard-WSL2-x86_64-with-glibc2.35`.

## Limits

- This is a single warm authorization scenario, not `button_confirm`, inventory reservation, a full purchase workflow, or a concurrent-load test.
- The two paths intentionally perform authorization only; timing mutations would add unequal business work.
- Raw output does not yet capture CPU model, so it does not close the broader benchmark-metadata gate.
- Use the raw distributions, not the mean-derived throughput, for tail-latency conclusions.

## Raw evidence

[`ODOO_ORM_COMPARISON_2026_09_15.json`](./ODOO_ORM_COMPARISON_2026_09_15.json) contains all 1,500 latency observations, method metadata, platform, and commit.
