# Production Readiness Checklist

> **Status:** **BLOCKED — P0 findings are open**
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

- [ ] README identifies the repository as a prototype until all P0 gates pass.
- [ ] “44,000x” is removed or replaced with a real Odoo measurement and raw data.
- [ ] Core latency, in-process latency, gRPC latency and ERP transaction latency are reported separately.
- [ ] Every benchmark records commit, Go version, OS, CPU, command, sample count and raw output.
- [ ] “E2E”, “WORM”, “encrypted”, “cluster-wide” and “production-ready” are used only after their corresponding gates pass.

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
- [~] Revision and NOTIFY errors abort transactions in code; injected PostgreSQL fault evidence remains open.
- [x] Older/duplicate events cannot downgrade an in-memory revision.
- [~] Role inheritance is persisted, revisioned and bundle-loaded; restart/catch-up evidence remains open.
- [x] A reconcile loop detects missed notifications without requiring a connection failure.

## 6. G4 — Revocation consistency

- [x] Revocations are namespaced by tenant and grant.
- [ ] Revocations survive pod restart for at least the grant validity window.
- [ ] Every serving replica observes a revoke before the documented deadline.
- [x] TTL cleanup bounds process-local memory growth.
- [ ] Tests cover at least three replicas, concurrent checks, restart and delayed delivery.
- [ ] Propagation p50/p95/p99 and maximum delay are published separately from local lookup time.

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
- [~] PostgreSQL durability is exercised in integration tests; encryption, spill/replay and retention remain open.
- [ ] Audit entries are tamper-evident and carry request/trace/revision identifiers.
- [ ] Key storage, rotation, recovery and separation of duties are documented.

## 9. G7 — Protocol and compatibility

- [x] One canonical `proto/v1/policy.proto` remains; the unused v0 contract was removed.
- [x] Go and Python clients are generated reproducibly from the same contract with pinned Buf/plugins.
- [x] Standard protobuf gRPC works without an undocumented JSON subtype.
- [ ] Backward-compatibility checks run in CI.
- [x] Deadlines/message-size limits are fail-fast configured; oversized wire requests and status semantics are tested.

## 10. G8 — Deployment and operations

- [ ] All testbed images are pinned by digest.
- [x] Compose references repository-local Odoo addon/generated-client assets; the Odoo image builds and installs the addon in a fresh isolated database.
- [ ] Environment variable names match application code and are tested.
- [x] `APP_ENV=production` enables strict secret, TLS and endpoint validation.
- [ ] Containers run as non-root with a read-only filesystem and least capabilities where practical.
- [ ] Readiness includes DB/sync/revision health; liveness does not hide degraded authorization state.
- [x] Shutdown cancels listeners/workers before waiting and completes within the grace period.
- [ ] Backup, restore, policy rollback, key rotation and incident runbooks have been rehearsed.

## 11. G9 — Test and performance evidence

- [~] Unit tests, `go vet` and formatting checks pass on audited commit `e16fa57`.
- [ ] Race detector passes on the release commit.
- [ ] Critical server, storage and security branches have negative tests and an agreed coverage gate.
- [~] Mandatory local E2E runs Odoo, PostgreSQL, PDP and the real PEP without `t.Skip` fallback (7/7 plus concurrency pass); a successful remote CI run and the combined Control Plane gate remain open.
- [ ] Benchmarks include worst-case global/same-leaf density and hash-collision handling.
- [ ] Production-path load test includes JWT, HMAC, metrics, audit and gRPC transport.
- [ ] Report includes p50/p95/p99/p99.9, throughput, errors, CPU, RSS and GC behavior.
- [ ] Real Odoo baseline executes ORM/database operations rather than `time.sleep`.

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
