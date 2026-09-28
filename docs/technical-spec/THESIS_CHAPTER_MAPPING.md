# Thesis Chapter Mapping

**Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

*Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17*

Updated 2026-09-28 for V2-WRITE-03. [Draft Chapters 3–5](../thesis-proposal/THESIS_CHAPTERS_3_5_DRAFT.md) follow this map; they are not the final submitted thesis.

## Research line

AI agent means a non-human software principal using controlled human delegation. The empirical boundary is one Odoo 17 purchase-order confirmation path. Three contribution layers: delegation, exact transaction binding, and commit-time enforcement. Go PDP performance supports implementation. SAP remains applicability discussion only.

Trusted Odoo records supply CBI fields; the agent supplies a proposal. PDP decisions are ALLOW/DENY, with REQUIRE_HUMAN_APPROVAL an ALLOW obligation. Pending approval releases locks; final execution revalidates under locks and consumes approval/command atomically with the protected ERP mutation. A DENY cannot be overridden.

## Chapter and RQ ownership

| Chapter | Main content | Evidence or source |
|---|---|---|
| 1 — Problem and scope | Agent tool actions, bounded authorization problem, RQ1–RQ4, three contributions | [Master plan](../thesis-proposal/THESIS_V2_MASTER_PLAN.md), [proposal](../thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md) |
| 2 — Background and threats | ABAC/delegation/SoD, attacker and trust boundary, related work; no novelty claim from naming alone | [Threat model](THREAT_MODEL.md), [security invariants](SECURITY_INVARIANTS.md) |
| 3 — Model and design | RQ1 human/agent/tenant/scope; RQ2 CBI/witness/versioning; RQ3 capability, lifecycle and consumption | [CBI contract](CANONICAL_BUSINESS_INTENT.md), [approval contract](APPROVAL_CAPABILITY.md) |
| 4 — Odoo implementation | Go/Odoo protocol, exact money, direct/pending/approved routes, publication and local-authority fences, deferred expiry, rollback/retry | Current source/tests, [audit](CURRENT_STATE_AUDIT.md), [addon guide](../../custom_addons/pdp_authorizer/README.md) |
| 5 — Evaluation and limitations | Bounded answers to RQ1–RQ3; A/B/C and separated measurements for RQ4; threats to validity and SAP applicability | [Matrix](EVALUATION_MATRIX.md), [EVAL-01 ledger](evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md), [EVAL-02 report](evidence/V2_EVAL_02_COMPARISON_2026_09_27.md), [EVAL-03 measurements](evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md), [EVAL-04 claim analysis](evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) |

## What can be written as a result now

VERIFIED V2 includes CBI/proof, AC v1, SoD, protected final revalidation and same-transaction consumption, with composed evidence for the 25 retained matrix IDs. The 2026-09-27 clean gate reports 75 post-tests, zero failures/errors, plus separate concurrency/authority/material/expiry runners. These counts describe different units; do not sum them or call all IDs independent ERP E2E proofs.

Explain the configured-writer, intact-trigger and trusted-clock assumptions beside the commit invariant. Deferred deadline validation is not a WAL/network-time guarantee. Manual publication recovery and coarse-epoch contention remain unevaluated; the earlier transient UNAVAILABLE remains recorded.

EVAL-02 now supplies 11 scenarios × 3 variants with committed-state oracles. Chapter 4 should explain its deferred-line-flush repair and three-transaction approval regression; Chapter 5 must retain the explicit A/B ablation limits and native retry result. This is correctness evidence, not measured latency.

EVAL-03 supplies separate Go API/control and Odoo workflow distributions with raw hashes and environment metadata. Chapter 5 must keep timer overhead/quantization, inclusive spans, different direct/approved workloads and single-writer execution beside the values. EVAL-04 supplies the permitted C1–C9 claims, bounded RQ answers and construct/internal/external/conclusion validity threats.

## Writing status

- V2-WRITE-03 cross-document reconciliation is complete; the active proposal, draft, evidence boundaries and references use the same title, scope and contribution framing.
- V2-WRITE-04 generated the final proposal DOCX/PDF and recorded provenance. The five-page PDF was rendered and visually inspected successfully; DOCX visual rendering remains open because the configured workspace bundle has no LibreOffice executable.

Do not reuse historical engine latency as current transaction latency. Do not infer production readiness, SAP compatibility, HMAC non-repudiation or exactly-once external effects. See [scope alignment](../thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) for the full evidence boundary.
