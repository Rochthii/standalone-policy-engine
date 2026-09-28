# Thesis V2 Atomic Task Board

> **Status (2026-09-28):** All nine thesis tasks and paired proposal artifact QA are complete. A separately authorized post-freeze authorization-semantics follow-up is tracked below; it does not reopen or change the thesis-plan completion status. The DOCX matches the shared Markdown source; all five pages of the existing paired PDF passed visual inspection. Per user scope decision, Word-native DOCX rendering is not a separate remaining gate; its lack of independent rendering remains a documented limitation, not a blocker.
> **Execution policy:** one dependency-complete task at a time; batch related acceptance gaps without nested task IDs or per-case approval turns. Use `ACTIVE_TASK.md` as the resume pointer and its ledger for detailed evidence. Consolidate validation and documentation at the batch boundary; acceptance and authorized Git publication are separate.
> **Master plan:** [`THESIS_V2_MASTER_PLAN.md`](./THESIS_V2_MASTER_PLAN.md)

## Status legend

| Status | Meaning |
|---|---|
| `TODO` | Not started. |
| `IN PROGRESS` | Current bounded task. |
| `CODE COMPLETE` | Focused checks pass; boundary evidence remains. |
| `VERIFIED — DOCX QA OPEN` | Historical status used before the 2026-09-28 paired-artifact closure decision. |
| `VERIFIED` | Required evidence passed and was recorded. |
| `BLOCKED` | Named dependency or decision is missing. |

## Phase 0 — Direction lock

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-DOC-00 | Archive the current proposal V1 Markdown, DOCX and PDF with a version index; do not delete or overwrite the baseline. | — | `VERIFIED` | `archive/v1-2026-09-19/` preserves Markdown, two historical DOCX files and PDF; version index and master-plan link verified. |
| V2-DOC-01 | Replace title, thesis statement and RQ1–RQ4 in proposal Markdown. | V2-DOC-00 | `VERIFIED` | Active proposal now states the Odoo 17 one-hop purchase-order scope, transaction-bound thesis statement and aligned RQ1–RQ4; performance is supporting evidence in RQ4. |
| V2-DOC-02 | Rewrite research gap, contributions, scope, 16-week plan and five-chapter outline. | V2-DOC-01 | `VERIFIED` | Proposal now defines the transaction-integrity gap, four measurable contributions, Odoo-first scope, a 16-week plan and five chapters mapped to RQ1–RQ4; unsupported dynamic HR/limit claims are absent. |
| V2-DOC-03 | Reconcile scope/evidence alignment, chapter mapping and bounded Odoo-to-SAP applicability mapping with v2. | V2-DOC-02 | `VERIFIED` | Both files now distinguish VERIFIED BASELINE from PLANNED V2, align to RQ1–RQ4 and limit SAP to applicability mapping with no implementation/effectiveness claim. |
| V2-DOC-04 | Update proposal generator; regenerate and visually verify DOCX/PDF. | V2-DOC-03 | `VERIFIED — PAIRED ARTIFACT QA` | DOCX matches all 80 Markdown blocks and three tables. Existing paired PDF was rendered with bundled Poppler 26.07.0 (`pdftoppm.exe -png -r 180 <proposal.pdf> <temp-prefix>`); all five A4 pages inspected without clipping, overlap, missing glyphs or broken tables. Per user scope decision, Word-native DOCX pagination is not a separate remaining gate; it was not independently rendered. No new export or content change. |

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
| V2-APP-04 | Invalidate changed, expired, revoked or already-consumed approval. | V2-APP-03 | `VERIFIED` | A locked Odoo revalidation operation reconstructs current CBI, checks local grant lifecycle, verifies stored AC v1 through the PDP and rechecks the original approver's current local/PDP authority. Fresh-database tests make changed intent, expired capability/grant, revoked grant and lost approver role terminal without a protected effect; existing terminal states cannot reopen, while transient PDP outage fails closed without permanent invalidation. Actual consumption and final mutation remain Phase 4 work. |

### Phase 3 primary files

- `custom_addons/pdp_authorizer/models/authorization_attempt.py`
- `custom_addons/pdp_authorizer/models/approval_request.py`
- `custom_addons/pdp_authorizer/models/purchase_order.py`
- One narrowly scoped approval model/module if extending the attempt model would mix responsibilities.
- Approval proof verifier and focused tests.

## Phase 4 — Commit-time enforcement

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-TXN-01 | Inventory and close final purchase-order transition entry points in scope. | V2-APP-04 | `VERIFIED V2` | Pinned Odoo Community public `button_approve`, `button_done`, `button_unlock` and direct `write(state=...)` cannot bypass the PEP for a sticky delegated-scope PO; focused 6-post-test/8-case and full 26-post-test/34-case gates pass. |
| V2-TXN-02 | Lock/re-read the row and reconstruct intent immediately before final execution. | V2-TXN-01 | `VERIFIED V2 PREFLIGHT` | Private same-transaction preflight locks PO/approval/attempt/lines, compares exact approved CBI and invalidates material drift without a final effect; 30 post-tests / 38 cases pass. Integration into final execution remains V2-TXN-03. |
| V2-TXN-03 | Recheck current PDP policy/revocation and atomically consume approval/command with mutation. | V2-TXN-02 | `VERIFIED V2 — BOUNDED FINAL ROUTE` | Public approved `button_confirm` reuses locked CBI/AC/current-authority checks and a fresh agent PDP `ALLOW` (approval obligation only on ALLOW), then marks approval `consumed`, executes the PO transition and marks the attempt `executed` in one Odoo transaction. Any `DENY`, including DENY with an approval obligation, remains non-final. Full fresh-database gate: 38 post-tests / 48 cases, 0 failures/errors; final-session races/rollback/retry remain TXN-04. |
| V2-TXN-04 | Add rollback, concurrent execution, lost-response retry and outage cases. | V2-TXN-03 | `VERIFIED V2 — BOUNDED PO` | Fresh-database gate: 41 post-tests / 51 cases, 0 failures/errors. Injected post-transition rollback and final-route AC/agent outages leave approval and command retryable; two independent final sessions converge on one consumed approval/attempt/PO effect, and a new-session retry does not call `button_approve` again. External effects and concurrent business-edit races are not covered. |

## Phase 5 — Evaluation

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-EVAL-01 | Materialize and finalize 20–30 bounded adversarial cases from the designed matrix. | V2-TXN-04 | `VERIFIED V2 — BOUNDED, PUBLICATION PENDING` | [Final ledger, 2026-09-27](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md): all 25 retained IDs anchored at their composed boundaries. One fresh gate exits 0: 75 post-tests, zero failures/errors, existing concurrency/retry, 16 material, three grant-ordering, three committed-authority, four after-ALLOW policy/role and two deferred-expiry schedules. Shared publication/epoch fences close retained TXN-N03 ordering; exact configured-writer/clock/trigger limits remain. Real PostgreSQL writer/failure tests and Go regressions pass. No commit/push authorized. |
| V2-DOC-05 | Reconcile the locked VN/EN title, scope, agent definition, trusted-intent flow, three contributions and bounded evidence across named docs; derive DOCX/PDF from the aligned Markdown and visually check where supported. | V2-EVAL-01 | `VERIFIED — PAIRED ARTIFACT QA` | Eight overview/proposal sources aligned; one 80-block Markdown source; DOCX/PDF content parity passed. Existing PDF's five pages were re-rendered at 180 dpi and inspected. User accepted paired-artifact QA as closure; direct Word pagination remains a non-blocking, explicitly unverified detail. No runtime claim expanded. |
| V2-EVAL-02 | Define and run variants A: broad service account, B: policy-only, C: proposed mechanism. | V2-EVAL-01, V2-DOC-05 | `VERIFIED V2 — BOUNDED` | [2026-09-27 report](../technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md): 11 shared scenarios × 3 variants, 33 committed/fresh-observer outcomes. Explicit isolated A/B ablations, real PDP and ordinary approval; no normal-route bypass. Repaired deferred line flush causing false CBI invalidation across three transactions. ABC passes; final fresh regression passes 75 post-tests and all runners. Five failed diagnostics retained; no timing claim or Git publication. |
| V2-EVAL-03 | Measure evaluator, proof, gRPC/mTLS, state check and ERP mutation separately. | V2-EVAL-02 | `VERIFIED V2 — BOUNDED` | [Contract/results](../technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md): corrected Go full run has 1,000 successful samples per boundary; summary validates accepted Go and retained Odoo runs. Raw hashes, timer-control distribution, QPC caveat and claim limits recorded. Failed diagnostics preserved. |
| V2-EVAL-04 | Record claim-to-evidence results and threats to validity. | V2-EVAL-03 | `VERIFIED V2 — BOUNDED` | [Claim/evidence ledger](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md): nine permitted claim families, bounded RQ1–RQ4 answers and construct/internal/external/conclusion validity threats mapped to EVAL-01/02/03. Threat model and writing pointers reconciled; no new runtime evidence or broad ERP/production/SAP/compliance claim. |

### V2-DOC-05 target and guardrail

- **Edit/reconcile:** `README.md`, `docs/00_MASTER_INDEX.md`, active proposal Markdown, `THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`, `docs/technical-spec/THESIS_CHAPTER_MAPPING.md`, `SE_ERP_CAREER_ROADMAP.md`, `custom_addons/pdp_authorizer/README.md` and `scripts/generate_master_thesis_proposal.py`.
- **Regenerate after source alignment:** active proposal DOCX/PDF; preserve the archived V1 artifacts unchanged.
- **Use as evidence authority, not copy-edit target:** source/protocol/tests, `CURRENT_STATE_AUDIT.md`, normative technical contracts and the case ledger. Update their implementation claims only through the owning evidence task. Keep `CHANGELOG.md` chronological and append-only.
- Inventory all README and active thesis overview/title/evidence references for contradictions. Do not mechanically rename the repository or rewrite subsystem specifications whose technical purpose and claims remain correct.

## Phase 6 — Thesis freeze

| ID | Atomic task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-WRITE-01 | Update current-state audit and task evidence from passed tests only. | V2-EVAL-04 | `VERIFIED` | Added a concise superseding audit entry linked to EVAL-03/04, including denominators, dirty-worktree and trust assumptions, and prohibited generalization. Relative links/status and scoped whitespace checks pass. Docs-only; no runtime rerun. |
| V2-WRITE-02 | Rewrite Chapters 3–5 around model, enforcement and results. | V2-WRITE-01 | `VERIFIED — DRAFT COMPLETE` | [Chapters 3–5 draft](THESIS_CHAPTERS_3_5_DRAFT.md) covers contracts/trust/invariants, Odoo/Go execution, EVAL-01/02/03, bounded RQ1–RQ4 answers and validity threats. Relative links resolve; 13 Odoo and four Go metric rows match the accepted report; 11 A/B/C scenario rows retained. No export or runtime rerun. |
| V2-WRITE-03 | Final cross-document reconciliation of proposal, chapter mapping, references and quantitative claims after the locked framing and evaluation results are incorporated. | V2-WRITE-02 | `VERIFIED — BOUNDED` | Proposal/draft/map agree on title, RQ1–RQ4, three contributions, Odoo-only runtime scope and SAP discussion boundary. Fixed stale EVAL-03 status in README/index/master plan and obsolete next-work text; checked cited source metadata, result units/limits and local links. No evidence/runtime claim added. |
| V2-WRITE-04 | Regenerate/render final DOCX/PDF and preserve reproduction metadata. | V2-WRITE-03 | `VERIFIED — PAIRED ARTIFACT QA` | DOCX matches all 80 Markdown blocks and three tables; five-page PDF rendered and all pages inspected at 180 dpi without clipping/overlap/missing glyphs or broken tables. Exact command, renderer, dirty source revision and hashes are in CHANGELOG.md. Word-native pagination was not independently rendered and is not a blocker per user scope decision. |

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

No task remains in the original nine-task thesis plan. Paired proposal artifact QA is complete; no further export is needed. A future supervisor requirement for Word-specific pagination can be handled as a new task.

## Locked remaining execution order

The original nine-task order is retained below; all nine tasks are complete. Paired proposal artifact QA is closed as a final handoff check, not an additional thesis task.

| Order | Task | Work and handoff |
|---:|---|---|
| 1 | `V2-EVAL-01` — complete | Bounded criteria accepted, clean fresh-database gate passed; publication pending explicit authorization. |
| 2 | `V2-DOC-05` — complete | Eight source docs aligned; single-source generator and content/link checks pass; paired DOCX source parity and five-page PDF visual QA recorded. |
| 3 | `V2-EVAL-02` — complete | 33 bounded comparison outcomes; exact ablations/raw evidence recorded; CBI flush repair and final regression pass. |
| 4 | `V2-EVAL-03` — complete, bounded | Go and Odoo boundary-separated distributions accepted with raw hashes and explicit interpretation limits. |
| 5 | `V2-EVAL-04` — complete, bounded | Nine claim families and RQ answers mapped to evidence with explicit validity threats and prohibited generalizations. |
| 6 | `V2-WRITE-01` — complete | Current-state audit, task board, checkpoint and changelog reconciled to bounded EVAL-03/04 evidence. |
| 7 | `V2-WRITE-02` — draft complete | Three chapters in one evidence-linked Markdown source; RQ1–RQ4 and validity threats covered. |
| 8 | `V2-WRITE-03` — complete, bounded | Proposal, chapter draft, map, references, evidence limits and current-status summaries reconciled; stale planned-performance and next-work text corrected. |
| 9 | `V2-WRITE-04` — complete | Regenerated DOCX/PDF, checked exact DOCX source parity, visually inspected all five PDF pages and recorded output provenance. |

**Artifact limitation:** Word-native DOCX pagination was not independently rendered in this environment. The existing paired PDF was rendered and visually inspected, and DOCX content parity with the shared Markdown source is recorded. This limitation is closed as non-blocking under the user's explicit paired-artifact scope decision; do not describe it as Word-rendered evidence.

## Post-freeze follow-up

| ID | Task | Depends | Status | Exit evidence |
|---|---|---|---|---|
| V2-CURRENCY-01 | Enforce the delegation grant ceiling and currency-scoped monetary policy for the bounded Odoo PO path. | User-approved review findings after thesis-plan closure | `COMPLETE — BOUNDED V2 EVIDENCE` | Exact CBI minor units are checked against the explicit grant currency/ceiling on every protected intent reconstruction. Exact cap succeeds; excess cap and PO/grant currency mismatch deny without mutation; USD is explicit in seeded Cedar/SQL rules and EUR/omitted currency deny. `go test ./...` and focused currency policy regression pass. The clean-source Odoo gate is recorded in [baseline evidence](../technical-spec/evidence/CLEAN_BASELINE_GATE_2026_09_28.md); existing-database migration is covered by V2-CURRENCY-MIG-01, while custom policy configuration remains unverified. |
| V2-CURRENCY-MIG-01 | Verify and, if required, safely migrate pre-existing Odoo delegation grants to explicit currency-bound ceilings. | V2-CURRENCY-01 | `COMPLETE — BOUNDED MULTI-COMPANY UPGRADE VERIFIED` | A dedicated disposable Odoo 17/PostgreSQL database simulated the pre-currency schema with existing active USD/EUR grants under separate delegator companies. Actual `--update=pdp_authorizer` retained exact maxima/state and assigned each grant its delegator company's currency; manifest version advanced 17.0.5.0.0→17.0.6.0.0; a second update preserved a deliberately selected alternate currency. The clean-source full gate passed on `29afd70550645a8ba780345dbb4531fe7638b38a`: 78 post-tests (0 failures/errors), mTLS and all configured concurrency/authority/material-race runners; [full provenance](../technical-spec/evidence/CLEAN_BASELINE_GATE_2026_09_28.md). This does not rehearse a customer backup or all historical upgrade paths. |
