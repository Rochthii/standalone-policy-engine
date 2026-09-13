# Implementation Task Board

> **Updated:** 2026-09-13
>
> **Current release status:** **BLOCKED — NOT PRODUCTION READY**
>
> This board decomposes [`REMEDIATION_EXECUTION_PLAN.md`](./REMEDIATION_EXECUTION_PLAN.md) into reviewable tasks. `VERIFIED` means the listed evidence was executed; it does not imply the parent release gate is complete.

## Status legend

| Status | Meaning |
|---|---|
| `TODO` | Work has not started. |
| `IN PROGRESS` | Code or test work is active. |
| `CODE COMPLETE` | Code compiles/focused tests pass, but required boundary evidence is unavailable. |
| `VERIFIED` | Required automated evidence for this task passed. |
| `BLOCKED` | A named artifact, decision or safe test environment is missing. |

## Wave 0 — Evidence baseline

| ID | Atomic task | Status | Evidence / blocker |
|---|---|---|---|
| DOC-001 | Establish authoritative current-state audit and release checklist. | `VERIFIED` | Docs linked and `git diff --check` clean. |
| DOC-002 | Create dependency-ordered remediation plan and task board. | `VERIFIED` | This board and execution plan are linked from the master index. |
| CI-001 | Require format, vet, unit and compile-only jobs on every change. | `VERIFIED` | Workflow has build, compile-only, vet, format, race-unit and focused delegation gates; local vet/compile pass. |
| CI-002 | Provide a non-destructive isolated PostgreSQL integration test database. | `VERIFIED` | Unique owned database harness and PostgreSQL 15 CI service added; local PostgreSQL 15 integration passes. |
| CI-003 | Run race tests on a runner with a C compiler. | `BLOCKED` | Local Windows runner has no GCC for CGO. |

## Wave 1 — Identity and delegation boundary

| ID | Atomic task | Status | Evidence / remaining work |
|---|---|---|---|
| SEC-AUTH-01 | Require Bearer JWT and mandatory `sub`, `tenant_id`, `exp`; restrict algorithm to HS256. | `VERIFIED` | Negative tests and `go test ./internal/security ./internal/server ./tests` pass. |
| SEC-CTX-02 | Make signed JWT principal attributes authoritative; remove ambiguous evaluator raw-key fallback. | `VERIFIED` | Forged namespaced and raw principal-attribute tests pass. |
| SEC-REST-03 | Apply the authenticated tenant/subject boundary to REST decision fallback. | `VERIFIED` | Missing token, body tenant/subject spoof and attribute-forgery tests pass. |
| SEC-IDOR-04 | Scope policy Get/Update/Delete/Publish SQL by tenant. | `VERIFIED` | Real PostgreSQL cross-tenant Get/Update/Publish/Delete isolation matrix passes. |
| SEC-REVOKE-05 | Authenticate Revoke RPC; bind `revoked_by` to JWT and require `delegation:revoke`. | `VERIFIED` | Missing token/permission, forged subject and cross-tenant tests pass. |
| SEC-PROOF-06 | Replace five-field HMAC with versioned length-prefixed full-tuple proof; require proof for delegated calls. | `VERIFIED` | Complete-tuple tamper matrix, expiry and max-TTL tests pass. |
| SEC-PROOF-07 | Define replay/idempotency semantics; add key-id rotation and end-to-end nonce enforcement. | `VERIFIED` | Key-ring/Go-Python tests plus real Odoo duplicate, altered-command replay, rollback and two-session serialization-retry cases pass; concurrency yields one nonce, one attempt and one mutation. |
| SEC-CONFIG-08 | Centralize production JWT/delegation secrets and issuer/audience validation. | `VERIFIED` | Production config matrix and issuer/audience JWT tests pass. |
| SEC-RBAC-09 | Require explicit signed permissions for Control Plane routes. | `VERIFIED` | `policy:read/write/simulate/operate` middleware tests pass. |
| SEC-TLS-10 | Require cert/key/client-CA configuration in production and pass it through centralized config. | `VERIFIED` | Production config tests and full repository compile pass. |
| SEC-TLS-E2E-11 | Run the Odoo -> PDP boundary with real server/client certificates and client-CA verification in the frozen testbed. | `VERIFIED` | Ephemeral test CA/server/client certificates are generated per run; missing-client-cert and wrong-hostname probes are rejected, a valid client reaches the JWT boundary, and all seven Odoo cases plus concurrency pass over mTLS. |
## Wave 2 — Policy and revocation consistency

| ID | Atomic task | Status | Exit evidence |
|---|---|---|---|
| CONS-COW-01 | Make COW writers linearizable across load/update/unload. | `VERIFIED` | Concurrent multi-tenant and same-tenant writer stress tests pass; read path remains lock-free and benchmark stays at 0 allocs/op. |
| CONS-COMPILE-02 | Compile a tenant ruleset all-or-nothing and preserve last-known-good state. | `VERIFIED` | Invalid batch and last-known-good preservation tests pass. |
| CONS-REV-03 | Make ACTIVE→DRAFT, publish and delete revision/event changes atomic. | `CODE COMPLETE` | Real PostgreSQL happy-path passes; injected revision/NOTIFY rollback failure evidence remains. |
| CONS-EVENT-04 | Fail transactions on revision/NOTIFY errors; handle duplicate/out-of-order/gap events. | `VERIFIED` | Stale, duplicate and late-sync event tests pass. |
| CONS-ROLE-05 | Persist, revision and load role inheritance with policies. | `CODE COMPLETE` | Real PostgreSQL bundle/DAG tests pass; engine restart and missed-event catch-up evidence remains. |
| CONS-RECON-06 | Add periodic reconciliation independent of listener reconnect. | `VERIFIED` | Cancellation and healthy-listener periodic reconciliation tests pass. |
| REV-SCOPE-01 | Namespace revocation by tenant + grant and bound TTL cleanup. | `VERIFIED` | Cross-tenant collision, expiry cleanup, concurrent access and server tests pass. |
| REV-DURABLE-02 | Persist revocation and propagate it to every replica. | `VERIFIED` | PostgreSQL snapshot-first LISTEN sync, fail-closed degraded state, three replicas, concurrent checks, delayed delivery and restart pass under the 5s SLO. See `evidence/REVOCATION_DURABILITY_2026_09_13.md`. |

## Wave 3 — Obligations, Odoo and audit

| ID | Atomic task | Status | Exit evidence |
|---|---|---|---|
| OBL-MODEL-01 | Define typed obligation representation and policy validation. | `VERIFIED` | Typed DSL parse plus unsupported/duplicate compiler rejection tests pass. |
| OBL-ENGINE-02 | Synthesize obligations in engine decisions and serialize structured payloads. | `VERIFIED` | Engine guardrail tests no longer mutate results; structured gRPC/REST obligation build and focused tests pass. |
| ODOO-PEP-01 | Migrate/version the Project 2 addon to the current standard Protobuf, JWT/mTLS, full-tuple proof and typed-obligation contract; preserve the non-rollback PEP. | `VERIFIED` | The repository addon installs in a fresh Odoo 17 database and the real transaction suite verifies standard Protobuf/JWT/full-tuple proof, typed obligations, nonce outcomes, non-rollback approval and mTLS transport. |
| ODOO-E2E-02 | Run real Odoo transaction cases: allow, hard deny, approval obligation, PDP outage, revoked grant, tampered proof and replay/concurrency. | `VERIFIED` | `make test-odoo-e2e` ran Odoo ORM -> generated gRPC client -> live Go PDP -> PostgreSQL: 7 tests with 0 failures/errors plus a passing two-session concurrency/retry assertion. See `evidence/ODOO_E2E_2026_09_12.md`. |
| AUD-REDACT-01 | Define and enforce proof/credential/PII redaction before output. | `VERIFIED` | Credential/proof/PII leak-negative tests pass; audit benchmark remains 0 allocs/op. |
| AUD-PIPE-02 | Wire bounded async queue to durable sink with observable failure policy. | `VERIFIED` | Queue-full, blocked-sink, graceful flush and real PostgreSQL CopyFrom tests pass. |
| AUD-CRYPT-03 | Implement key lifecycle, encryption, spill/replay and tamper evidence. | `VERIFIED` | Versioned KEK rotation, AAD/HMAC tamper rejection, atomic encrypted spill/restart replay and PostgreSQL idempotency tests pass. See `evidence/AUDIT_CRYPTO_DURABILITY_2026_09_13.md`. |

## Wave 4 — Contract and runtime

| ID | Atomic task | Status | Exit evidence |
|---|---|---|---|
| API-IDL-01 | Select one canonical proto and reproducibly generate Go/Python clients. | `VERIFIED` | Buf-pinned Go/Python generation, Go Docker wire E2E and generated Python-to-live-Go compatibility pass. |
| API-RUNTIME-02 | Define codec, deadlines, limits and error semantics. | `VERIFIED` | Standard protobuf wire E2E, fail-fast runtime config and oversized-request ResourceExhausted test pass. |
| OPS-CONFIG-01 | Prove every environment key has a runtime consumer. | `TODO` | Configuration-consumer test passes. |
| OPS-IMAGE-02 | Make testbed self-contained and pin images by digest. | `IN PROGRESS` | The repository-local Odoo/PDP images build and the isolated E2E gate passes; base-image digest pinning remains open. |
| OPS-HEALTH-03 | Make readiness report policy/sync/dependency health. | `TODO` | Degraded pods leave readiness before receiving traffic. |
| OPS-SHUTDOWN-04 | Cancel listeners/workers before waiting and bound shutdown. | `VERIFIED` | Blocking listener is canceled and `Stop` completes within the test deadline. |
| EDGE-RESTORE-05 | Implement snapshot restore or remove edge-startup claim. | `TODO` | Offline restart test or explicit scope removal. |

## Wave 5 — Release evidence

| ID | Atomic task | Status | Exit evidence |
|---|---|---|---|
| PERF-WORST-01 | Benchmark dense candidate sets and hash-collision correctness. | `TODO` | Raw repeatable samples with allocation data. |
| PERF-FULL-02 | Measure JWT + proof + gRPC + metrics + audit path. | `TODO` | p50/p95/p99/p99.9, throughput, CPU/RSS/GC. |
| PERF-ODOO-03 | Replace synthetic Odoo baseline with real ORM/database work. | `TODO` | The real Odoo E2E boundary now passes; define an equal-workload comparative benchmark and remove the sleep/hardcoded model from evidence. |
| REL-GATE-04 | Execute every production-readiness gate and record GO/NO-GO. | `TODO` | All P0 closed; release evidence linked to one commit. |

## Next task

Next: prove every configured environment key has a runtime consumer (`OPS-CONFIG-01`). Remote CI inspection remains tracked by `CI-003`/G9.
