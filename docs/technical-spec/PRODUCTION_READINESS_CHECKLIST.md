# Production Readiness Checklist

> **Status:** **BLOCKED — release gates remain open**
>
> **Source of findings:** [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md)
>
> A release is production-ready only when every mandatory gate below has executable evidence attached to a commit or release artifact.

## 1. Gate policy

Status values:

- `[ ]` not verified
- `[~]` partially implemented; insufficient evidence
- `[x]` verified by an automated release gate

No gate may be marked `[x]` using comments, screenshots, manually edited result files, simulated sleeps, or unit tests that bypass the production boundary under test.

## 2. G0 — Claim and documentation integrity

- [x] README identifies the repository as a prototype until every release gate passes.
- [x] The “44,000x” claim is retired and replaced with real Odoo/PostgreSQL purchase-confirmation versus mTLS-PDP raw data; see `evidence/ODOO_ORM_COMPARISON_2026_09_15.md`.
- [x] Core latency, in-process latency, gRPC latency and ERP transaction latency are reported separately; the ERP boundary includes the business mutation and explicitly excludes the final database commit.
- [x] Every benchmark records commit, Go version, OS, CPU, command, sample count and raw output.
- [x] “E2E”, “WORM”, “encrypted”, “cluster-wide” and “production-ready” are qualified by their corresponding evidence boundary.

## 3. G1 — Identity and tenant isolation

- [x] Every Data Plane and Explain RPC rejects a missing authorization credential.
- [x] JWT `sub`, `tenant_id`, `exp`, issuer and audience are mandatory and validated.
- [x] Only the configured signing algorithm is accepted.
- [x] Production startup fails if JWT/HMAC secrets or required mTLS material are absent/default.
- [x] The real Odoo boundary passes with hostname-verified server TLS, a trusted client certificate and rejection of clients without a certificate.
- [x] Trusted identity attributes cannot be overridden by request context.
- [x] REST fallback decisions enforce the same trust boundary as gRPC or are removed.
- [x] Negative tests cover missing token, missing claims, wrong issuer/audience and cross-tenant access.

## 4. G2 — Delegation authorization

- [x] Delegated requests require a proof; omission fails closed.
- [x] Canonical encoding is versioned and unambiguous.
- [x] Proof binds tenant, grant, delegator, agent, action, resource/scope, constraints, chain, validity window and nonce.
- [x] Proof validation enforces a maximum lifetime and a versioned `kid` key ring with overlap/retirement tests.
- [x] Resource creator and other SoD attributes are cryptographically bound.
- [x] Revoke RPC authenticates the caller and verifies tenant ownership plus revoke permission.
- [x] Cross-tenant, cross-action, delimiter-confusion and altered-chain tests pass; real Odoo duplicate, altered-command, rollback and two-session serialization-retry evidence also passes.

## 5. G3 — Policy state correctness

- [x] COW writes are linearizable under concurrent lazy load, sync, update and unload operations.
- [x] A stress test proves that concurrent updates cannot lose tenants or revisions; race-detector evidence remains in G9.
- [x] Policy compilation is all-or-nothing; an invalid policy preserves the last-known-good ruleset.
- [x] ACTIVE -> DRAFT, publish and delete atomically update revision and emit an event.
- [x] PostgreSQL revision-trigger and notifier-SQL fault injection proves revision/notification failures abort publish, ACTIVE -> DRAFT and delete transactions without changing policy state or tenant revision.
- [x] Older/duplicate events cannot downgrade an in-memory revision.
- [x] Role inheritance is persisted, revisioned and bundle-loaded; a fresh engine rebuilds the PostgreSQL-backed DAG and revision reconciliation replaces it after a deliberately missed event.
- [x] A reconcile loop detects missed notifications without requiring a connection failure.

## 6. G4 — Revocation consistency

- [x] Revocations are namespaced by tenant and grant.
- [x] Revocations survive pod restart for at least the grant validity window.
- [x] Every serving replica observes a revoke before the documented 5-second deadline or fails delegated checks closed while synchronization is unavailable.
- [x] TTL cleanup bounds process-local memory growth.
- [x] Tests cover three replicas, concurrent checks, restart and deliberately delayed delivery over real PostgreSQL.
- [x] Propagation p50/p95/p99 and maximum delay are published separately from local lookup time in `evidence/REVOCATION_DURABILITY_2026_09_13.md`.

## 7. G5 — Decision obligations and PEP behavior

- [x] Policies have an explicit obligation representation and validation rules.
- [x] Decision synthesis attaches obligations without test-side mutation.
- [x] gRPC and REST preserve structured obligation payloads, not only string types.
- [x] Odoo PEP writes `to approve` without raising and records the nonce outcome in the same PostgreSQL transaction; the real Odoo suite verifies one state transition and one Activity.
- [x] Real Odoo transaction tests cover allow, hard deny, deny-with-approval and PDP failure; revoked and tampered proof cases also pass.

## 8. G6 — Audit durability and confidentiality

- [x] Sensitive context fields and proofs are redacted before entering the audit queue.
- [x] The production main uses the configured PostgreSQL audit writer.
- [x] Queue capacity, overflow policy and backpressure behavior are explicit.
- [x] Writer failures and drops have counters and tests.
- [x] PostgreSQL durability, envelope encryption and idempotent spill/replay are exercised against PostgreSQL 15 and restart/key-rotation tests.
- [x] Audit entries authenticate metadata and ciphertext and carry request/trace/revision identifiers.
- [x] Key storage, rotation, recovery and separation of duties are documented in `RUNBOOK_OPS.md`.
- [ ] Retention, external append-only archival and deletion-evidence policy are defined and rehearsed.

## 9. G7 — Protocol and compatibility

- [x] One canonical `proto/v1/policy.proto` remains; the unused v0 contract was removed.
- [x] Go and Python clients are generated reproducibly from the same contract with pinned Buf/plugins.
- [x] Standard protobuf gRPC works without an undocumented JSON subtype.
- [x] Backward-compatibility checks run in CI; see `evidence/CI_COMPAT_2026_09_18.md`.
- [x] Deadlines/message-size limits are fail-fast configured; oversized wire requests and status semantics are tested.

## 10. G8 — Deployment and operations

- [x] All external testbed images are pinned by manifest-list digest; repository-local PDP/Odoo images are built from those pinned bases and the full Odoo mTLS E2E gate passes.
- [x] Compose references repository-local Odoo addon/generated-client assets; the Odoo image builds and installs the addon in a fresh isolated database.
- [x] Environment variable names match application code and the complete runtime binding matrix is tested.
- [x] `APP_ENV=production` enables strict secret, TLS and endpoint validation.
- [x] PDP and Control Plane images run as non-root; Kubernetes templates require read-only roots, dropped capabilities and explicit writable mounts. See `evidence/OPS_DEPLOY_2026_09_18.md`.
- [x] Readiness includes DB/sync/revision health; liveness does not hide degraded authorization state. See `evidence/OPS_HEALTH_2026_09_17.md`.
- [x] Shutdown cancels listeners/workers before waiting and completes within the grace period.
- [x] Backup, restore, policy rollback, key rotation and incident runbooks were rehearsed in an isolated environment; see `evidence/OPS_RUNBOOK_2026_09_18.md`.

## 11. G9 — Test and performance evidence

- [x] Unit tests, `go vet` and formatting checks pass on audited commit `12458f0`; see `evidence/CI_ODOO_2026_09_18.md`.
- [x] Race detector passes on audited commit `cc5f5d2` in the GitHub-hosted Ubuntu `Unit Tests` job; see `evidence/CI_RACE_2026_09_17.md`.
- [x] Critical server, storage and security branches have explicit negative tests and CI coverage floors (45%/20%/70%); see `evidence/CI_COVERAGE_2026_09_18.md`.
- [x] Mandatory Odoo and combined Control Plane E2E gates run PostgreSQL, PDP and the real PEP without a CI skip fallback; remote CI passed on `cffcd51`. See `evidence/CI_ODOO_2026_09_18.md` and `evidence/REL_INTEGRATION_2026_09_18.md`.
- [x] Benchmarks cover 10,000 global/same-leaf candidates and a forced collision-bucket test validates raw-key handling; see `evidence/DENSE_CANDIDATE_COLLISION_2026_09_14.md`.
- [x] Local TCP application-path measurement validates JWT, HMAC, metrics, encrypted audit queueing and gRPC transport; mTLS and PostgreSQL audit flush remain excluded by scope. See `evidence/FULL_PATH_2026_09_14.md`.
- [x] The local application-path report records p50/p95/p99/p99.9, throughput, zero errors, CPU, RSS and GC behavior; see `evidence/FULL_PATH_2026_09_14.md`.
- [x] Real Odoo baseline executes a record-rule ORM/PostgreSQL authorization rather than `time.sleep`; see `evidence/ODOO_ORM_COMPARISON_2026_09_15.md`.

## 12. Recommended execution order

1. Security boundary: G1 and G2.
2. State correctness: G3 and G4.
3. Business behavior and evidence: G5 and G6.
4. Integration foundation: G7 and G8.
5. Release proof: G9, followed by final updates to G0.

Performance micro-optimization should resume only after G1–G6 pass. The core evaluator already has sufficient headroom; correctness and evidence are the limiting factors.

## 13. Release decision record

| Release | Commit | Decision | Open P0 | Evidence owner | Date |
|---|---|---|---:|---|---|
| Current audit baseline | `e16fa57` | BLOCKED | 12 | Unassigned | 2026-09-11 |
