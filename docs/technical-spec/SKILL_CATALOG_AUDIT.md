# Skill Catalog Audit & Redesign Plan

> **Audit date:** 2026-09-11
>
> **Scope:** The nine repository-local skills under `.agents/skills/`.
>
> **Authority:** This document complements [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md). A local skill must never state an implementation claim that contradicts the current-state audit.
>
> **Verdict:** Keep the catalog small, but do not keep it unchanged. One skill is currently sound, two are mostly sound with corrections, and six need substantive rewrites. Five new, narrowly scoped skills close the material gaps.

## 1. Why the current catalog is risky

The catalog is short and discoverable: all nine entrypoints are 29–44 lines. That is good. Its main problem is not length; it is that several skills encode the intended architecture as completed fact. An agent following those instructions can preserve optional authentication, weak proof binding, process-local revocation, synthetic benchmarking, or an audit pipeline that is not actually connected.

The skills must guide a future implementation toward the audited release gates, not teach it to reproduce historical claims.

## 2. Existing-skill disposition

| Skill | Decision | Why | Required change |
|---|---|---|---|
| `critical-advisor` | KEEP | It is short, discriminating and asks for evidence, operational cost and alternatives. | Add a pointer to the current-state audit when reviewing this repository. |
| `clean-architecture-standards` | REWRITE LIGHTLY | Central configuration and separation are useful; the absolute 250-line rule can create artificial fragmentation and it omits configuration-consumer tests. | Make 250 lines a review threshold, require one config owner and tests proving each env key is consumed. |
| `dsl-compiler` | REWRITE LIGHTLY | Grammar, depth cap and pre-parsing guidance are useful. | Add parser input-size/fuzz guidance, atomic ruleset compilation, type/error reset behavior and truthful `contains` allocation caveat. |
| `engine-evaluator` | REWRITE | It calls nested hash maps a Trie with `O(log N)`, promises zero allocation too broadly, and omits writer linearizability plus hash collision verification. | Separate narrow benchmark guarantees from production behavior; require CAS/serialization for writers and worst-case candidate benchmarks. |
| `agent-authorization` | REWRITE URGENTLY | It teaches the current weak five-field HMAC, optional proof check and local revocation as security controls. | Make JWT mandatory, bind the full tuple, require trusted attributes, authorize revocation, prevent replay and distinguish local lookup from cluster propagation. |
| `grpc-dataplane` | REWRITE URGENTLY | It says tenant isolation is strict while the handler accepts missing credentials; it also points to an absent external Odoo client. | Define actual interceptor order, mandatory auth, mTLS policy, deadlines, structured errors and generated-contract compatibility. |
| `storage-audit` | SPLIT AND RETIRE | It combines two large concerns and describes an unimplemented ring buffer/CopyFrom/spill pipeline as fact. | Replace with `policy-state-consistency` and `audit-pipeline`; keep no catch-all router. |
| `erp-testing` | REWRITE URGENTLY | It labels in-process vectors E2E and repeats the invalid 44,000x Odoo claim. | Require real boundary tests, negative security cases, multi-replica revocation tests, and honest benchmark methodology. |
| `docker-standards` | REWRITE | It calls mutable tags frozen, assumes a nonexistent external addon and lacks Kubernetes runtime security. | Require digest pinning, self-contained assets, security contexts and a release-safe E2E workflow. |

## 3. Skills that must be added

### 3.1 `policy-state-consistency` — P0

**Trigger:** policy CRUD, revision/NOTIFY, policy sync, lazy loading, role data, COW state, cache eviction, PostgreSQL migration or reconciliation changes.

**Why it is missing:** No current skill owns the security-critical transition from persistent policy state to in-memory decision state.

**Must enforce:** tenant-scoped transactions; all-or-nothing compile; monotonic revisions; event failure handling; duplicate/out-of-order event rules; writer linearizability; singleflight load; role inheritance persistence; tests for stale active policy and lost updates.

**Boundary:** It does not design audit delivery or optimize evaluator instructions.

### 3.2 `audit-pipeline` — P0

**Trigger:** decision auditing, log encryption/redaction, queueing, persistence, Vector/ClickHouse/Kafka, spill/replay, retention or key rotation.

**Why it is missing:** The current `storage-audit` skill describes an audit architecture that the production path does not execute.

**Must enforce:** no credential/proof plaintext; explicit queue/backpressure/loss policy; durable sink semantics; encryption key lifecycle; replay idempotency; failure metrics; integration evidence for a blocked sink and recovery.

**Boundary:** It does not own policy revision consistency.

### 3.3 `api-contract-governance` — P0

**Trigger:** `.proto` edits, generated stubs, JSON codec, gRPC/Python/Odoo client compatibility, RPC versioning, deadlines and wire-level E2E tests.

**Why it is missing:** A handwritten Go stub plus two divergent proto files make contract drift and client incompatibility likely.

**Must enforce:** one canonical versioned IDL; reproducible generation; a documented codec decision; compatibility checks; message-size/deadline/error conventions; generated Go and Python client tests against a live server.

**Boundary:** Authentication policy remains in `agent-authorization` and `grpc-dataplane`.

### 3.4 `documentation-evidence-governance` — P0

**Trigger:** README, benchmark, roadmap, thesis, acceptance matrix, architecture claim, release note or any “verified/E2E/production-ready” statement.

**Why it is missing:** Documentation previously presented targets and synthetic values as completed empirical facts.

**Must enforce:** claim-to-artifact traceability; separate design targets from implemented behavior; explicit scope for every metric; raw command/output metadata; no simulated baseline presented as measured; synchronized index/status updates when a claim changes.

**Boundary:** It does not generate thesis prose or perform benchmark execution itself.

### 3.5 `kubernetes-runtime-hardening` — P1

**Trigger:** Kubernetes manifests, Envoy, probes, secrets, TLS, service exposure, resource limits, deployment rollout, containers or production operational runbooks.

**Why it is missing:** Docker image reproducibility and Kubernetes runtime security are different concerns; the current Docker skill covers neither sufficiently.

**Must enforce:** digest pinning; non-root/read-only runtime; secret delivery; required TLS; NetworkPolicy/service exposure; readiness based on authorization health; graceful shutdown; PDB/HPA decisions; manifest validation and smoke tests.

**Boundary:** It does not own local Compose test fixtures.

## 4. Conditional skills: add only when the artifact exists

| Skill | Add when | Scope |
|---|---|---|
| `odoo-pep-integration` | The Odoo addon is imported into this repository or a versioned submodule. | PID-safe client lifecycle, trusted ERP attributes, transaction/non-rollback semantics, activity scheduling and Odoo tests. |
| `dependency-supply-chain` | A release pipeline and published images/modules exist. | `govulncheck`, SBOM, dependency update policy, image signing and provenance. |

Creating either skill before its artifact/workflow exists would add speculative instructions rather than useful guidance.

## 5. Target catalog after the first remediation wave

| Domain | Skill | Source |
|---|---|---|
| Architecture | `clean-architecture-standards` | Rewrite existing |
| Critical review | `critical-advisor` | Keep, audit-aware update |
| Policy language | `dsl-compiler` | Rewrite existing |
| Evaluation core | `engine-evaluator` | Rewrite existing |
| Delegation identity | `agent-authorization` | Rewrite existing |
| Data Plane runtime | `grpc-dataplane` | Rewrite existing |
| Policy consistency | `policy-state-consistency` | New |
| Audit delivery | `audit-pipeline` | New |
| API contract | `api-contract-governance` | New |
| ERP verification | `erp-testing` | Rewrite existing |
| Containers/testbed | `docker-standards` | Rewrite existing |
| Kubernetes runtime | `kubernetes-runtime-hardening` | New |
| Claims/evidence | `documentation-evidence-governance` | New |

This produces 13 skills. That is the upper useful limit for the current repository: each has a distinct trigger and owner. Do not add generic “security”, “performance”, “DevOps”, “testing”, or “Go best practices” skills; they would overlap existing system guidance and reduce routing precision.

## 6. Common rules for every revised local skill

1. Start with a discriminating trigger and explicit non-goals.
2. Link to `CURRENT_STATE_AUDIT.md` and relevant release gates; never embed a historical metric as a permanent invariant.
3. State current implementation gaps separately from the desired end state.
4. Require observable verification appropriate to the risk: negative security test, race/stress test, integration test, or recorded benchmark.
5. Use repository-relative paths; do not hardcode `file:///E:/...` paths or point to an absent external project.
6. Keep `SKILL.md` to routing and non-obvious invariants. Put substantial schemas/checklists in a focused `references/` file only when a task needs them.
7. Never instruct an agent to mark a target “verified” merely because a unit test passes.

## 7. Implementation order

1. Rewrite `agent-authorization`, `grpc-dataplane`, `erp-testing`, `storage-audit`, and `engine-evaluator` before using them for new production work.
2. Add `policy-state-consistency`, `audit-pipeline`, `api-contract-governance`, and `documentation-evidence-governance`.
3. Rewrite `docker-standards` and add `kubernetes-runtime-hardening` before the next deployment attempt.
4. Apply the light rewrites to `clean-architecture-standards`, `dsl-compiler`, and `critical-advisor`.
5. Add `odoo-pep-integration` only after the addon becomes a versioned repository artifact.

Each new or substantially revised skill must be validated with the system skill validator and forward-tested against one realistic task. The expected behavior—not wording—is the validation target.

