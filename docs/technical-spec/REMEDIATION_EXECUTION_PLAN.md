# Remediation Execution Plan

> **Status:** Active execution plan — consult the task board for per-item evidence.
>
> **Authority:** This plan turns the P0/P1 findings in [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md) and the release gates in [`PRODUCTION_READINESS_CHECKLIST.md`](./PRODUCTION_READINESS_CHECKLIST.md) into an executable order of work. It does not change the current status: **NOT PRODUCTION READY**.

## 1. Decision rule

The project must be hardened in dependency order, not in the order that produces attractive benchmark numbers. A fast evaluator is irrelevant if a caller can forge identity, replay a delegation or observe stale policy state.

- Do not release, connect to a real ERP transaction path, or claim E2E/production readiness while any P0 finding remains open.
- Every work item needs an owner, pull request, negative test or integration test as appropriate, and an evidence link in the release record.
- Do not mark a finding fixed from a unit test that bypasses the production boundary being repaired.
- Keep last-known-good policy state on all compilation, database or notification failures; never publish a partial ruleset.

## 2. Workstream map

| Wave | Objective | Findings / release gates | Exit evidence |
|---|---|---|---|
| 0 | Establish a truthful, testable baseline | DOC-001; G0; TEST-002 | CI records the audited commands, negative-test skeletons exist, and release claims are traceable. |
| 1 | Close identity and delegation bypasses | SEC-001…SEC-005; G1–G2 | Live gRPC/REST negative tests prove missing/forged/cross-tenant credentials fail closed. |
| 2 | Make policy and revocation state correct under failure/concurrency | CONS-001…CONS-003; REV-001; G3–G4 | Stress and multi-replica tests prove no lost update, no partial publish and bounded revoke propagation. |
| 3 | Deliver the business outcome and durable audit | OBL-001; AUD-001; INT-001; G5–G6 | Real Odoo PEP, obligations and encrypted durable audit work together across processes. |
| 4 | Make the protocol and runtime deployable | API-001; OPS-001…OPS-004; EDGE-001; G7–G8 | Canonical generated contract, reproducible images, valid runtime probes and recovery drills. |
| 5 | Produce release evidence and revisit performance | PERF-001…PERF-002; G9 | Reproducible full-path load, E2E and race results meet explicitly agreed SLOs. |

## 3. Detailed execution backlog

### Wave 0 — Baseline and guardrails

1. Make the audit, checklist and this plan required review inputs for security, deployment and benchmark changes.
2. Add CI jobs for formatting, `go vet`, unit tests, race tests on a runner with a C compiler, and focused negative-security tests.
3. Create an evidence record template: commit, environment, command, raw output, pass/fail and reviewer.
4. Label all existing in-process delegation tests as integration/unit scope; remove any release gate that treats them as Odoo E2E.

**Done when:** G0 is traceable and the release job fails when a required security/E2E evidence artifact is absent.

### Wave 1 — Identity, tenancy and delegation (P0)

1. Require authenticated identity for every Data Plane and Explain RPC. Validate `sub`, `tenant_id`, expiry, issuer, audience and the configured algorithm; fail startup without non-default secret or required TLS material.
2. Define one trusted-attribute merge policy: client context cannot overwrite token-derived tenant, subject, role or delegation attributes.
3. Replace the delegation proof with a versioned canonical encoding binding tenant, grant, delegator, agent, action, resource/scope, constraints, chain, expiry and nonce. Require it for delegated calls.
4. Add replay protection, maximum TTL and key-id/key rotation rules.
5. Protect `RevokeDelegation`; verify caller identity, tenant ownership and revoke permission.
6. Scope every Control Plane read/mutation query by authenticated tenant; add authorization roles before exposing policy administration.

**Mandatory tests:** no token, malformed token, missing tenant claim, wrong issuer/audience, tenant mismatch, request-attribute overwrite, altered action/resource/chain, replay, unauthorized revoke and cross-tenant policy-ID access.

**Done when:** G1 and G2 are fully checked using a live server and a real database where relevant.

### Wave 2 — State consistency and revocation (P0)

1. Choose and document a linearizable writer model for engine state (CAS retry loop or explicit serialized writer). Cover lazy load, sync, unload and publish together.
2. Make policy compilation and state publication all-or-nothing. Preserve last-known-good tenant rules on invalid input, database failure or event failure.
3. Make ACTIVE-to-DRAFT, publish and delete update revision and notification atomically in one tenant-scoped transaction.
4. Persist, revision and load role inheritance with policies.
5. Replace process-local revocation with tenant-scoped, durable state plus cross-replica propagation. Bound retention by grant expiry and specify a maximum propagation delay.
6. Add periodic reconciliation for missed notifications and correct shutdown cancellation ordering.

**Mandatory tests:** concurrent writer stress/race, stale-ACTIVE transition, invalid-forbid compile, duplicate/out-of-order/gap events, process restart, three-replica revoke propagation and delayed notification recovery.

**Done when:** G3 and G4 pass; no test can observe a lost update, partial ruleset or an unbounded/restart-lost revocation within the declared model.

### Wave 3 — Obligations, Odoo PEP and audit (P0)

1. Define typed obligation policy syntax and decision payloads; synthesize obligations in the engine rather than mutating test results.
2. Maintain the repository-owned Odoo addon and verify its non-rollback `to approve` behavior, nonce ledger and fail-closed paths with real Odoo transactions.
3. Wire the production audit path to its configured durable sink. Redact or encrypt sensitive material before it leaves the PDP.
4. Specify queue capacity, overflow behavior, retry/backpressure, spill/replay, idempotency, retention and encryption-key lifecycle.
5. Test a blocked sink, recovery, restart, tamper detection and audit search/reconciliation.

**Dependency:** The addon source is now present, but Wave 3 cannot close until the real Odoo transaction and audit durability evidence passes.

**Done when:** G5 and G6 pass in a real PDP + PostgreSQL + Odoo environment.

### Wave 4 — Contract, deployment and operability (P1)

1. Select a single canonical versioned `.proto`; generate Go and Python/Odoo clients reproducibly. Remove or explicitly version stale contracts and document codec, deadlines, limits and errors.
2. Reconcile every environment variable with an application consumer test; eliminate unused or contradictory configuration.
3. Make Compose/E2E self-contained, pin all images by digest and remove destructive cleanup from normal verification.
4. Harden runtime deployment: non-root user, read-only filesystem where feasible, least privileges, TLS/secret delivery and network exposure rules.
5. Make readiness represent authorization health, policy revision lag and essential dependencies. Test bounded graceful shutdown.
6. Either implement edge snapshot restoration with recovery tests or remove the unsupported edge-startup claim.

**Done when:** G7 and G8 pass through a clean-environment deployment and restore/rollback exercise.

### Wave 5 — Evidence, performance and release decision (P1)

1. Add worst-case policy-density, candidate-list and hash-collision benchmarks; state clearly where linear scans remain or eliminate them.
2. Measure the complete path separately from the core engine: JWT/HMAC, gRPC transport, metrics, audit and database/ERP flow.
3. Run real Odoo ORM/database baselines. Never use `time.Sleep` or hardcoded PDP outcomes as performance evidence.
4. Record p50/p95/p99/p99.9, throughput, errors, CPU, RSS, GC, hardware and raw samples.
5. Close the release checklist and make an explicit GO/NO-GO decision. Update README, thesis and benchmarks only from this evidence.

**Done when:** G9 passes and all P0 findings are closed. Production readiness is a release decision, not an inferred label.

## 4. Critical decisions the project owner must make

These choices determine implementation design; do not silently choose them inside a feature patch.

| Decision | Why it matters | Required before |
|---|---|---|
| Target deployment topology and replica count | Defines revocation propagation, reconciliation and availability tests. | Wave 2 |
| Identity provider, issuer/audience and mTLS boundary | Defines verifier configuration, key rotation and service-to-service trust. | Wave 1 |
| Authoritative source for resource/creator/role attributes | Determines whether SoD inputs can be trusted or must be proof-bound. | Wave 1 |
| Audit retention, privacy and acceptable loss policy | Determines storage, encryption, queue/spill and operational cost. | Wave 3 |
| Odoo ownership and versioned delivery method | Required to build a reproducible PEP/E2E environment. | Wave 3 |
| Release SLOs for security, revocation and latency | Prevents optimizing against unapproved targets. | Wave 5 |

## 5. Skill work aligned to the plan

Before each wave, use skills only after they have been made audit-accurate as defined in [`SKILL_CATALOG_AUDIT.md`](./SKILL_CATALOG_AUDIT.md).

| Before wave | Required skill remediation |
|---|---|
| 1 | Rewrite `agent-authorization` and `grpc-dataplane`; add `api-contract-governance`. |
| 2 | Rewrite `engine-evaluator`; replace `storage-audit` with `policy-state-consistency`. |
| 3 | Add `audit-pipeline`; rewrite `erp-testing`. Add `odoo-pep-integration` only after importing the addon. |
| 4 | Rewrite `docker-standards`; add `kubernetes-runtime-hardening`. |
| 5 | Add `documentation-evidence-governance`; lightly correct `dsl-compiler` and `clean-architecture-standards`. |

## 6. First implementation slice

The smallest safe next slice is **Wave 1 only**: mandatory authenticated gRPC identity, tenant-scoped request handling, complete delegation-proof binding and authorization for revocation. It should ship as one coherent security change with its negative test matrix, not as scattered individual patches.

Do **not** start further micro-optimizations, claimed Odoo benchmarks, or deployment publication until this slice is complete.
