# Thesis Scope and Evidence Alignment

**Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

*Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17*

Updated 2026-09-28 under V2-EVAL-04. This alignment interprets accepted EVAL-01/02/03 evidence; it does not certify a new runtime build.

## Scope and contribution

An AI agent is a non-human software principal invoking business tools under controlled one-hop human delegation. The research boundary is authorization for one Odoo 17 `purchase.order` confirmation path. Model intelligence, full ERP protection and SAP execution are outside scope.

Three contribution layers are delegation-aware authorization, transaction binding and commit-time enforcement. Go PDP/Trie/DAG/AST, gRPC and mTLS are implementation mechanisms. RQ4 evaluates these contributions, not a fourth mechanism or a claim to the fastest policy engine.

The agent proposes an action. Trusted Odoo records supply canonical business fields. PDP returns ALLOW or DENY; REQUIRE_HUMAN_APPROVAL is an obligation on ALLOW only. The pending route releases locks before waiting. Final execution locks/rereads, checks exact intent and current authority, then consumes required approval/command with the ERP mutation. Activity is notification, not approval; DENY is never overridden.

## Evidence authority

Follow current source/protocol/executable tests, then [current audit](../technical-spec/CURRENT_STATE_AUDIT.md), then checkpoint/task board, V2 designs and historical material. The [EVAL-01 ledger](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) records concrete commands, source scope, failures and limits. Designed contracts do not by themselves establish implementation.

| Label | Meaning |
|---|---|
| VERIFIED BASELINE | Earlier tested boundary; not automatically the current V2 route |
| DESIGNED V2 | Normative design, distinct from passing runtime evidence |
| PLANNED V2 | Evaluation or implementation still to perform |
| VERIFIED V2 | Exact implemented boundary backed by recorded executable evidence |

## RQ-to-evidence map

| RQ | Current bounded support | Remaining interpretation/evaluation |
|---|---|---|
| RQ1: who may represent whom? | Trusted caller binding, same-tenant one-hop grant, scope/validity/constraints; policy/revocation and local-authority ordering in configured testbed | No multi-hop, arbitrary writer or external identity lifecycle guarantee |
| RQ2: is this the exact current transaction? | Go/Python CBI/proof vectors, persistent-state negatives, 16 ordered material-edit schedules, replay/retry and protected transition checks | Not a proof for every field or arbitrary Odoo extension; keep fixture and lock assumptions |
| RQ3: is required approval still valid and unused? | Purpose-separated AC v1, exact pending intent/witness, independent-human SoD, current approver authority, atomic consumption and rollback; approved/direct expiry cases | HMAC integrity only; no legal non-repudiation or external-effect exactly-once |
| RQ4: what improves and what does it cost? | EVAL-01 adversarial evidence, [EVAL-02's 33 bounded A/B/C outcomes](../technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md), and [EVAL-03's separated distributions](../technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md) | [EVAL-04](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) limits conclusions: no universal baseline ranking, causal approval overhead, speedup or production SLA |

## EVAL-01 closure

The 2026-09-27 fresh Odoo 17/mTLS/PDP/PostgreSQL gate passed with 75 post-tests and zero failures/errors. Independent runners separately cover concurrency/retry, 16 material schedules, three grant-ordering schedules, three committed-authority changes, four after-ALLOW policy/role schedules and two real-clock deferred-expiry rollbacks. Real PostgreSQL writer/failure and Go regressions are recorded in the ledger.

All 25 retained IDs have composed-boundary anchors. Do not add unlike runner/test counts into a new total or describe the 25 IDs as universal end-to-end proofs. Evidence was produced at the recorded commit plus dirty worktree, not at a released artifact.

## Limits that must accompany claims

- Every in-scope policy/revocation writer must share the configured ERP fence. Policy publication first marks readiness false; partial failure remains fail-closed.
- Local-authority triggers and the database UTC clock are trusted. Locks/fences order configured authority changes with the protected ERP transaction, not a distributed atomic transaction.
- Expiry is checked at PostgreSQL deferred validation, not at a later WAL durability or network-delivery instant.
- Coarse local-authority contention is unmeasured. Publication recovery remains manual and has no end-to-end recovery evidence.
- An earlier transient first-fence UNAVAILABLE was not fully diagnosed; the clean rerun is not a broad availability guarantee.
- No production readiness, instant/global revocation, legal compliance, general prompt-injection prevention or general ERP/SAP compatibility claim.
- Historical microbenchmarks do not establish current workflow overhead. Use only EVAL-03's named boundaries and caveats; do not subtract nested/control percentiles or infer causal approval overhead.

## Odoo-to-SAP applicability only

| Verified or designed Odoo concept | Question for a future SAP implementation |
|---|---|
| Human/agent/tenant identity binding | How are principal propagation and business authorization established in the chosen SAP service? |
| CBI and material witness | Which business-object fields and versions are authoritative? |
| Exact-action approval and SoD | Which workflow decision and approver rules can be bound to the same action? |
| Locks, fences and local atomic mutation | What transaction boundary and supported extension points can enforce equivalent invariants? |

These are transfer questions, not an implemented mapping or a compatibility result.

## Next work

EVAL-01/02/03 and the EVAL-04 claim analysis are complete at their bounded evidence levels. The Chapters 3–5 Markdown draft is available at [THESIS_CHAPTERS_3_5_DRAFT.md](THESIS_CHAPTERS_3_5_DRAFT.md); V2-WRITE-03 reconciled it against the proposal and evidence boundaries. V2-WRITE-04 regenerated the proposal DOCX/PDF and visually inspected the PDF. DOCX visual rendering remains open because this workspace has no bundled LibreOffice executable; the task board records the exact gate. Keep [V1](archive/v1-2026-09-19/VERSION_INDEX.md) unchanged.
