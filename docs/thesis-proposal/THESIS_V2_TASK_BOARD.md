# Thesis V2 Atomic Task Board

> **Status:** READY
> **Execution policy:** one task per turn; do not start a task whose dependencies are incomplete.
> **Master plan:** [`THESIS_V2_MASTER_PLAN.md`](./THESIS_V2_MASTER_PLAN.md)

## Status legend

| Status | Meaning |
|---|---|
| `TODO` | Not started. |
| `IN PROGRESS` | Current bounded task. |
| `CODE COMPLETE` | Focused checks pass; boundary evidence remains. |
| `VERIFIED` | Required evidence passed and was recorded. |
| `BLOCKED` | Named dependency or decision is missing. |

## Phase 0 — Direction lock

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-DOC-00 | Archive the current proposal V1 Markdown, DOCX and PDF with a version index; do not delete or overwrite the baseline. | — | `VERIFIED` | `archive/v1-2026-09-19/` preserves Markdown, two historical DOCX files and PDF; version index and master-plan link verified. |
| V2-DOC-01 | Replace title, thesis statement and RQ1–RQ4 in proposal Markdown. | V2-DOC-00 | `VERIFIED` | Active proposal now states the Odoo 17 one-hop purchase-order scope, transaction-bound thesis statement and aligned RQ1–RQ4; performance is supporting evidence in RQ4. |
| V2-DOC-02 | Rewrite research gap, contributions, scope, 16-week plan and five-chapter outline. | V2-DOC-01 | `VERIFIED` | Proposal now defines the transaction-integrity gap, four measurable contributions, Odoo-first scope, a 16-week plan and five chapters mapped to RQ1–RQ4; unsupported dynamic HR/limit claims are absent. |
| V2-DOC-03 | Reconcile scope/evidence alignment, chapter mapping and bounded Odoo-to-SAP applicability mapping with v2. | V2-DOC-02 | `VERIFIED` | Both files now distinguish VERIFIED BASELINE from PLANNED V2, align to RQ1–RQ4 and limit SAP to applicability mapping with no implementation/effectiveness claim. |
| V2-DOC-04 | Update proposal generator; regenerate and visually verify DOCX/PDF. | V2-DOC-03 | `CODE COMPLETE` | Generator emits V2 DOCX/PDF; PDF has two-page Poppler visual QA and content checks pass. DOCX visual render is blocked: bundled LibreOffice is absent and Word COM has no available logon session. Re-run DOCX render on a workstation with supported renderer before final submission. |

### Phase 0 files

- `docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md`
- `docs/thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`
- `docs/technical-spec/THESIS_CHAPTER_MAPPING.md`
- `scripts/generate_master_thesis_proposal.py`

## Phase 1 — Security model

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-MODEL-01 | Update trust boundaries, actors, threats and explicit assumptions. | V2-DOC-03 | `VERIFIED` | `THREAT_MODEL.md` maps T1-T10 to controls/tests; `SECURITY_INVARIANTS.md` states I1-I5 and explicit limits. |
| V2-MODEL-02 | Specify `CanonicalBusinessIntent v1` and authoritative source for every field. | V2-MODEL-01 | `VERIFIED` | Normative schema defines 20 sourced fields, exact minor-unit money, deterministic line digest/state witness, canonical encoding and approval-state lifecycle; implementation remains PLANNED V2. |
| V2-MODEL-03 | Specify `ApprovalCapability v1`, SoD, expiry, invalidation and key separation. | V2-MODEL-02 | `VERIFIED` | Normative AC v1 defines 19 sourced fields, independent-human SoD, PDP-side purpose-separated HMAC, lifecycle, invalidation and atomic consumption requirements; implementation remains NOT IMPLEMENTED. |
| V2-MODEL-04 | Update security invariants and evaluation matrix from the two schemas. | V2-MODEL-03 | `VERIFIED` | I1-I5 map to 25 named positive/negative cases with persistent-state oracles, bounded statuses and implementation task IDs. |

### Phase 1 files

- `docs/technical-spec/THREAT_MODEL.md`
- `docs/technical-spec/SECURITY_INVARIANTS.md`
- `docs/technical-spec/EVALUATION_MATRIX.md`
- `docs/thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`

## Phase 2 — Canonical intent and proof v2

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-PROOF-01 | Add versioned Go canonical intent/proof fields and validation. | V2-GOV-01 | `VERIFIED` | Go CBI/proof v2 tests accept the valid fixture and reject malformed fields, stale witness, material changes, key/version confusion and invalid validity; full `internal/security` regression passes. |
| V2-PROOF-02 | Implement matching Python/Odoo canonicalization using currency minor units. | V2-PROOF-01 | `VERIFIED` | Python rejects float/exponent/precision/range errors, validates top-level CBI/witness, and matches Go canonical bytes/hash for the bounded fixture; V1 protocol regressions pass. |
| V2-PROOF-03 | Add cross-language golden vector and material-field tamper matrix. | V2-PROOF-02 | `VERIFIED` | Shared multi-line fixture yields identical Go/Python line bytes/digest, witness, CBI bytes/hash and V2 proof; old proof rejects material intent/line tampering in focused Go tests. |
| V2-PROOF-04 | Require v2 for the high-impact path; document bounded v1 compatibility. | V2-PROOF-03, V2-GOV-02 | `VERIFIED` | Trusted persisted Odoo state produces CBI/proof V2; Go rejects missing, unknown, V1 and tampered high-impact requests before evaluation; the nine-test Odoo/mTLS gate and two-session nonce gate pass. V1 remains only at the tested nonnumeric legacy boundary. |

### Phase 2 primary files

- `internal/security/delegation_proof.go`
- `internal/security/canonical_purchase_order_lines.go`
- `internal/server/delegation_auth.go`
- `internal/server/delegation_auth_v2.go`
- `custom_addons/pdp_authorizer/cbi_protocol.py`
- `custom_addons/pdp_authorizer/cbi_lines.py`
- `custom_addons/pdp_authorizer/delegation_proof_v2.py`
- `custom_addons/pdp_authorizer/pdp_protocol.py`
- `custom_addons/pdp_authorizer/models/cbi_builder.py`
- `custom_addons/pdp_authorizer/models/delegation_grant.py`
- Focused Go/Python proof tests.

## Phase 3 — Exact-action approval

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-APP-01 | Persist pending canonical intent and one approval record/state. | V2-PROOF-04 | `VERIFIED` | Fresh-database Odoo/mTLS evidence persists one post-transition CBI JSON/canonical bytes/hash/witness row and one linked Activity; retry creates no duplicate and the PO remains non-final. |
| V2-APP-02 | Enforce authorized approver and SoD at approval creation. | V2-APP-01 | `VERIFIED` | Fresh-database Odoo/mTLS evidence derives the human from `env.user`; same-tenant/company purchase-manager plus live-PDP checks pass, while creator, delegator, agent, wrong-role, PDP-denied, cross-tenant/company and outage cases fail closed with pending state unchanged. |
| V2-APP-03 | Issue and verify a purpose-separated, expiring approval capability. | V2-APP-02 | `VERIFIED` | Typed AC v1 issue/verify RPCs use a dedicated key ring/domain and bounded TTL; Odoo locks and rechecks the unchanged pending intent, persists one `approved` capability, verifies it through the PDP and retries idempotently while the PO remains `to approve`. Tamper, expiry, unknown/key-confused credentials and non-human/SoD cases fail closed in focused and real-boundary tests. |
| V2-APP-04 | Invalidate changed, expired, revoked or already-consumed approval. | V2-APP-03 | `TODO` | Focused Odoo tests cover each invalidation cause. |

### Phase 3 primary files

- `custom_addons/pdp_authorizer/models/authorization_attempt.py`
- `custom_addons/pdp_authorizer/models/approval_request.py`
- `custom_addons/pdp_authorizer/models/purchase_order.py`
- One narrowly scoped approval model/module if extending the attempt model would mix responsibilities.
- Approval proof verifier and focused tests.

## Phase 4 — Commit-time enforcement

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-TXN-01 | Inventory and close final purchase-order transition entry points in scope. | V2-APP-04 | `TODO` | `button_confirm`, `button_approve` and supported API path cannot bypass PEP. |
| V2-TXN-02 | Lock/re-read the row and reconstruct intent immediately before final execution. | V2-TXN-01 | `TODO` | Material state mismatch requires new authorization/approval. |
| V2-TXN-03 | Recheck current PDP policy/revocation and atomically consume approval/command with mutation. | V2-TXN-02 | `TODO` | One database transaction contains consume and business state change. |
| V2-TXN-04 | Add rollback, concurrent execution, lost-response retry and outage cases. | V2-TXN-03 | `TODO` | One command ID produces at most one committed ERP effect. |

## Phase 5 — Evaluation

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-EVAL-01 | Materialize and finalize 20–30 bounded adversarial cases from the designed matrix. | V2-TXN-04 | `TODO` | Every negative case executes against its stated boundary and asserts no unauthorized persistent mutation. |
| V2-EVAL-02 | Define and run variants A: broad service account, B: policy-only, C: proposed mechanism. | V2-EVAL-01 | `TODO` | Same fixture, action and environment are used across variants. |
| V2-EVAL-03 | Measure evaluator, proof, gRPC/mTLS, state check and ERP mutation separately. | V2-EVAL-02 | `TODO` | Raw p50/p95/p99/max, errors and environment metadata are recorded. |
| V2-EVAL-04 | Record claim-to-evidence results and threats to validity. | V2-EVAL-03 | `TODO` | No general ERP, production, WORM or compliance conclusion is inferred. |

## Phase 6 — Thesis freeze

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-WRITE-01 | Update current-state audit and task evidence from passed tests only. | V2-EVAL-04 | `TODO` | New verified claims link executable evidence; failed/open gates stay explicit. |
| V2-WRITE-02 | Rewrite Chapters 3–5 around model, enforcement and results. | V2-WRITE-01 | `TODO` | Every RQ has a bounded answer and threats-to-validity section. |
| V2-WRITE-03 | Reconcile proposal, chapter mapping, references and final quantitative claims. | V2-WRITE-02 | `TODO` | Cross-document claim audit reports no contradiction. |
| V2-WRITE-04 | Regenerate/render final DOCX/PDF and preserve reproduction metadata. | V2-WRITE-03 | `TODO` | Visual QA passes and final evidence records commit, commands and environment. |

## Governance gate

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-GOV-01 | Align agent guidance, critical skills and changelog with V2 evidence authority. | V2-MODEL-04 | `VERIFIED` | Root and `.agents` guides are V2-aligned; V2-critical skills are syntax-validated, catalog status updated and changelog records the change. |
| V2-GOV-02 | Align `grpc-dataplane` skill before production-route V2 migration. | V2-PROOF-03 | `VERIFIED` | Skill reflects the real generated contract, handler/interceptor ownership, mandatory trust boundary, V1 compatibility limit and V2 no-downgrade gate; syntax/link/diff checks pass, and its bounded behavioral forward-test passed in V2-PROOF-04. |

## Priority boundary

### P0 — Required

Phases 0–5 and V2-WRITE-01 through V2-WRITE-03.

### P1 — Only after P0

- Policy revision surfaced explicitly in decision/audit.
- Revocation epoch.
- Optimistic concurrency comparison.
- A small second-workflow transfer validation.

### Deferred

- Multi-hop/multi-agent support.
- WebAuthn and cross-domain signatures.
- SAP, edge, external WORM archive and OPA/Cedar benchmarking.

## Next task

`V2-APP-04` — invalidate changed, expired, revoked or already-consumed approval without applying the protected purchase-order effect.
