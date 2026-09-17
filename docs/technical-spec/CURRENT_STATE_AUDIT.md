# Current-State Audit & Evidence Register

> **Audit date:** 2026-09-11
>
> **Audited commit:** `e16fa57` (`main`, tag `v1.0.0-core-verified`)
>
> **Remediation update:** The 2026-09-13 remediation closes the verified items recorded in [`IMPLEMENTATION_TASK_BOARD.md`](./IMPLEMENTATION_TASK_BOARD.md). The delegation proof has an explicit `kid` key ring, the repository-owned Odoo PEP has a transactional nonce ledger, revocations are persisted and propagated through PostgreSQL with snapshot-first startup/reconnect, policy publish/update/delete rollback is fault-injected against PostgreSQL, role inheritance is reloadable through the revision stream, and all testbed base images are manifest-digest pinned. The cross-language proof vector, seven fresh-database Odoo transaction tests, two-session serialization retry, real mTLS boundary probes, three-replica revocation/restart tests, six policy transaction rollback cases, real PostgreSQL role-DAG restart/catch-up cases, and the digest-pinned Odoo E2E testbed pass. Other named release gates remain open.
>
> **Authority:** This document describes the implementation that exists in this repository. Where an older architecture, roadmap, benchmark, or thesis document conflicts with this audit, this document takes precedence until the conflict is resolved and re-verified.
>
> **Overall status:** **NOT PRODUCTION READY**

## 1. Executive verdict

The repository contains a credible and fast in-memory policy evaluation core. The narrow evaluator benchmark is reproducible and currently achieves sub-microsecond decisions with zero reported heap allocations for the measured inputs.

The working tree now enforces the Data Plane identity boundary, key-identified full-tuple delegation proof, durable multi-replica revocation, tenant-scoped policy access, linearizable COW writers, atomic ruleset compilation, typed obligations, rotatable encrypted audit delivery with spill/replay, generated standard Protobuf clients, a constrained Badger edge-policy restore path, and a repository-owned Odoo PEP whose real transaction suite passes over mTLS. The system is still not production-ready because retention/external append-only archival, deployment hardening, race evidence and production-path load evidence remain open.

The correct positioning is:

- **Strong core-engine prototype** for continued research and hardening.
- **Incomplete distributed PDP product** that must not protect real ERP transactions yet.
- **Partially verified experiment**: the Go core benchmarks are valid for their scope; the real Odoo comparison covers one narrow warm authorization workload only and cannot support a general ERP speedup claim.

## 2. Status scorecard

| Area | Score | Current status |
|---|---:|---|
| In-memory evaluator | 8/10 | Strong narrow-path implementation and benchmark evidence |
| Parser and compiler | 7/10 | Functional depth cap and optimization; input/fuzz hardening remains |
| Decision correctness | 8/10 | Deny-default, forbid-overrides, atomic compilation and typed obligations are verified |
| Security boundary | 9/10 | JWT, tenant binding, RBAC, key-ring proof, mTLS and real Odoo duplicate/altered/rollback/two-session nonce outcomes are verified |
| Distributed consistency | 8/10 | COW/event/reconcile correctness plus durable snapshot-first revocation propagation pass locally; remote CI evidence remains open |
| Audit and compliance | 7/10 | Redacted envelope encryption, authenticated metadata, PostgreSQL idempotency and restart spill/replay pass; retention and deletion evidence remain open |
| Tests | 8/10 | Negative, PostgreSQL, protobuf, mTLS, seven-case Odoo and two-session retry coverage pass; race/load gates remain |
| Deployment | 5/10 | Repository-local images build from digest-pinned bases and the E2E profile passes; runtime hardening remains |
| Odoo integration | 9/10 | Fresh install, seven real ORM/mTLS-gRPC/PostgreSQL cases and two-session retry pass |
| Production readiness | 5/10 | Several P0s closed, but remaining security/durability/Odoo gates still block release |

## 3. Verification performed

| Check | Result |
|---|---|
| Focused unit/security/server/engine/storage suites + `go test -run=^$ ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt -l` on tracked Go files | PASS; no files reported |
| `git diff --check` | PASS |
| Race detector locally | NOT VERIFIED; local Windows toolchain has no GCC for CGO |
| Docker Compose configuration parse | PASS with obsolete `version` warning |
| Docker PostgreSQL + Control Plane + PDP protobuf E2E | PASS; Odoo is not included |
| Generated Python client -> live Go gRPC server | PASS with pinned `protobuf==6.32.1`, `grpcio==1.75.1` |
| Odoo/Python -> Go delegation proof golden vector | PASS |
| Odoo 17 fresh install + ORM -> generated gRPC client -> live PDP -> PostgreSQL | PASS over mTLS; missing client certificate rejected, valid client reaches JWT boundary, 7 tests with 0 failures/errors, plus two-session retry PASS |
| Three revocation replicas + delayed delivery + restart over PostgreSQL | PASS; 3 runs/108 propagation samples, worst observed 38.8256ms under the 5s SLO |
| Policy publish/update/delete revision and notifier rollback over PostgreSQL | PASS; six fault-injected transactions preserve policy state and tenant revision |
| Role-DAG restart and missed-event catch-up over PostgreSQL | PASS; a fresh engine rebuilds the persisted graph and revision reconciliation replaces it after an event is deliberately omitted |
| Digest-pinned Odoo/PostgreSQL/PDP mTLS testbed | PASS; all external bases resolved by manifest digest and seven Odoo cases plus concurrency pass |

Observed coverage:

| Package | Statement coverage |
|---|---:|
| `internal/security` | 81.4% |
| `internal/audit` | 78.6% |
| `internal/parser` | 68.3% |
| `internal/config` | 65.7% |
| `internal/engine` | 63.5% |
| `internal/server` | 33.8% |
| `internal/storage` | 20.6% |
| `pkg/pdp` | 8.7% |
| `internal/metrics` | 0.0% |
| `internal/pectl/commands` | 0.0% |

Observed Go core benchmarks on a 13th Gen Intel Core i7-13700H, Windows/amd64, Go 1.26.4:

| Benchmark | Three observed runs | Allocation |
|---|---:|---:|
| `BenchmarkEvaluatorLatency` | 390.3–492.8 ns/op | 0 B/op, 0 allocs/op |
| `BenchmarkConcurrentLoad` | 26.74–27.51 ns/op | 0 B/op, 0 allocs/op |
| `BenchmarkUltraExtreme_DeepDAG_HeavyABAC` | 839.8–845.1 ns/op | 0 B/op, 0 allocs/op |
| `BenchmarkUltraExtreme_10kPolicies_ConcurrentContention` | 34.30–67.11 ns/op | 0 B/op, 0 allocs/op |

Local full-path application evidence is recorded separately in `evidence/FULL_PATH_2026_09_14.md`: a persistent loopback TCP gRPC connection validates JWT, full-tuple HMAC proof, engine, metrics and encrypted audit queueing across three 10,000-request samples. It is not a mTLS, PostgreSQL-audit, Odoo, container-network or concurrent-load measurement.

`ns/op` from a parallel benchmark is aggregate throughput-normalized time. It must not be reported as the wall-clock latency of one network request.

## 4. Claim-to-evidence register

| Claim | Status | Evidence and required correction |
|---|---|---|
| Deny-by-default | VERIFIED | Implemented by the decision engine |
| Forbid-overrides | VERIFIED | Matching forbid short-circuits permit results |
| AST depth <= 15 | VERIFIED | Enforced during compilation |
| Core evaluator is sub-microsecond | VERIFIED FOR MEASURED CASES | Reproduced locally; scope is in-memory `Engine`, not gRPC E2E |
| Full application decision path is measured | VERIFIED FOR LOCAL LOOPBACK TCP CASE | Three 10,000-request samples include JWT, delegation proof, gRPC, metrics and encrypted audit queueing; no mTLS, PostgreSQL audit flush, Odoo or concurrent load |
| Zero allocations on every production hot path | PARTIAL | Measured cases pass; `contains`, normalization, buffer overflow, metrics, audit, JWT and gRPC are outside this guarantee |
| Zero linear policy scans | CONTRADICTED | Global and same-leaf candidate policies are iterated linearly |
| Strict multi-tenant JWT isolation | VERIFIED | Missing credentials/claims and cross-tenant requests fail; signed principal attributes are authoritative |
| Delegation proof prevents tuple tampering and replayed execution | VERIFIED | `v1.<kid>.<hmac>` plus the Odoo ledger pass key rotation, Go/Python compatibility, tamper, duplicate, altered-command, rollback and two-session serialization-retry cases |
| Durable cluster revocation | VERIFIED FOR LOCAL 3-REPLICA POSTGRESQL TEST | Snapshot-first LISTEN/reconnect preserves revocations across restart; delegated checks fail closed while sync is unavailable; worst observed propagation was 38.8256ms against a 5s SLO |
| Atomic policy revision/notification mutation | VERIFIED FOR LOCAL POSTGRESQL TEST | Revision trigger and notifier-SQL fault injection abort publish, ACTIVE -> DRAFT and delete without persisting a policy or tenant-revision change |
| Durable role inheritance reload | VERIFIED FOR LOCAL POSTGRESQL TEST | PostgreSQL-backed role graph is bundle-loaded by a fresh engine; reconciliation detects a later revision and atomically replaces the in-memory DAG |
| Runtime `REQUIRE_HUMAN_APPROVAL` obligation | VERIFIED | Compiler-valid typed obligations are synthesized by the deciding policy and serialized structurally |
| Bounded encrypted PostgreSQL audit delivery | VERIFIED FOR LOCAL POSTGRESQL TEST | Production main uses redaction, envelope encryption, idempotent batch merge and restart spill/replay; deletion evidence/retention remain open |
| Edge snapshot supports offline policy startup | VERIFIED FOR LOCAL BADGER RESTART TEST | Versioned policy source, role graph and revision snapshots are recompiled and atomically installed before the listener opens. Empty, legacy or invalid snapshots prevent startup. The edge profile has no durable revocation/audit backend, so delegated requests fail closed and it is not a production ERP profile. See `evidence/EDGE_RESTORE_2026_09_17.md`. |
| Odoo 17 PEP addon is included and verified | VERIFIED FOR SINGLE-PDP MTLS BOUNDARY | Fresh install, seven transaction cases and two-session retry pass through real ORM/mTLS-gRPC/PostgreSQL; clients without a certificate are rejected |
| Frozen testbed base images | VERIFIED FOR LOCAL E2E TESTBED | PostgreSQL 15, Go 1.25, Alpine 3.19 and Odoo 17 are manifest-digest pinned; local PDP/Odoo builds and the full Odoo mTLS E2E gate pass. Future image/CVE updates require an intentional digest refresh. |
| Readiness reports authorization dependencies | VERIFIED FOR LOCAL HTTP/CONFIG TESTS | `/readyz` verifies PostgreSQL, the active policy-sync listener and revision parity for every loaded tenant; `degraded` and `not_ready` return 503. Kubernetes probes this endpoint and Compose makes Odoo wait for PDP readiness. See `evidence/OPS_HEALTH_2026_09_17.md`. |
| 7/7 delegation vectors | VERIFIED AS IN-PROCESS TESTS | They do not prove Odoo/container/network integration |
| Odoo/PDP comparative authorization evidence | VERIFIED FOR ONE NARROW WARM WORKLOAD | The sleep/hardcoded model is retired. A real Odoo `ir.rule`/PostgreSQL check versus Odoo-to-PDP mTLS gRPC benchmark records 750 raw samples per path on commit `4bb4c48`; it shows a 1.346x mean difference but does not establish a general ERP speedup. See `evidence/ODOO_ORM_COMPARISON_2026_09_15.md`. |
| gRPC contract is standard generated Protobuf | VERIFIED | Buf-pinned Go/Python generation, Docker wire E2E and Python-to-Go live call pass |

## 5. Open findings

### P0 — Remaining release blockers

No previously identified P0 implementation finding remains open. Release is still blocked by incomplete P1/release gates and uninspected remote evidence.

### P1 — Required before release candidate

| ID | Finding | Impact |
|---|---|---|
| PERF-001 | Global/same-leaf policy lists are scanned linearly | Dense 10,000-policy cases are measured, but the zero-linear-scan invariant remains contradicted and arbitrary worst-case latency is not bounded |
| PERF-002 | Production boundaries remain outside the local application-path measurement | mTLS, durable PostgreSQL audit flush, revocation storage/synchronization, container networking, Odoo and concurrent load are not latency-bounded |
| TEST-001 | The mandatory Odoo/PostgreSQL/gRPC gate passes locally and is defined in CI, but no successful remote committed run has been inspected | The local result is valid evidence; release branch protection is not yet proven |
| DOC-001 | Current documents state verified facts that the repository contradicts | Academic and engineering credibility risk |

## 6. Documentation interpretation rules

- **Implemented** means a production path calls the feature.
- **Tested** means an automated test exercises that production path.
- **E2E tested** means the request crosses real process/network/storage boundaries and cannot silently skip in the release job.
- **Measured** means the reported value was produced by the described system, not injected through sleep, constants or estimated baselines.
- **Production-ready** requires every P0 finding closed and every release gate in `PRODUCTION_READINESS_CHECKLIST.md` passing.
- NIST, OWASP and SOX may be described as design mappings, not certification or compliance evidence.

## 7. Audit limitations

- The original audit did not mutate production code; the remediation update does include implementation changes and isolated PostgreSQL/Docker test systems.
- The PostgreSQL/Control Plane/PDP Docker E2E passes with standard protobuf. The repository-owned Odoo replacement also passes seven real fresh-database transaction cases and two simultaneous sessions with Odoo's standard serialization retry over mTLS.
- The Odoo run proves single-PDP PEP transaction/replay and mTLS behavior. Multi-replica revocation is evidenced separately by the local three-replica PostgreSQL integration test, not by the Odoo suite or a remote deployment.
- Race tests could not be reproduced locally without a C compiler. CI contains a race job, but a successful remote run was not independently inspected.
- Dependency vulnerability scanning, penetration testing and sustained network load testing remain outstanding.
- Audit envelope authentication detects modified metadata/ciphertext, but cannot prove record deletion or provide external WORM retention; that requires an independently controlled append-only archive.
