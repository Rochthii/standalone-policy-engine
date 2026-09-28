# Transaction-Bound Authorization for AI Agents in ERP

**Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

*Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17*

Research prototype for a Software Engineering graduation thesis, implemented on one Odoo 17 `purchase.order` confirmation path. Repository name: `standalone-policy-engine`. Author: Chăm Rốch Thi — PTIT.

## What this project investigates

Can an agent commit the exact ERP action a human delegated, with the required approval still valid and without replaying it?

An AI agent is modeled as a non-human software principal calling business tools under controlled human delegation. Model reasoning, training and prompt filtering are outside the evaluation.

Three contribution layers:

1. **Delegation-aware authorization:** human, agent, tenant, scope, validity and business constraints.
2. **Transaction binding:** authoritative canonical business intent, material-state witness and versioned integrity proof.
3. **Commit-time enforcement:** locked revalidation, exact-action human approval, and atomic one-time command/approval consumption with the protected ERP mutation.

Go PDP, policy indexing/role evaluation, gRPC and mTLS implement the mechanism. PDP speed is supporting evidence, not the thesis novelty.

## Protected execution flow

```text
Human delegation → Agent tool proposal
  → Odoo PEP reconstructs intent from authoritative ERP records
  → PDP: ALLOW or DENY
  → If ALLOW requires approval: persist "to approve"; release locks
  → Independent human approves the exact pending intent
  → Final execution: lock/reread, verify intent and current authority
  → Consume command/required approval + mutate PO in one ERP transaction
```

An ALLOW without the approval obligation follows the direct final route. `REQUIRE_HUMAN_APPROVAL` is an obligation on ALLOW, not a third decision; DENY is never overridden by a capability. An Odoo Activity is a notification, not approval evidence. Caller-supplied amount/vendor/lines are not authoritative business facts.

## Evidence and limits

**VERIFIED V2 — bounded evidence, 2026-09-27:** the fresh Odoo 17/mTLS/PDP/PostgreSQL gate passed with **75 post-tests, zero failures/errors**. Separate runners cover concurrency/retry, 16 material-edit schedules, grant ordering, authority changes and deferred-expiry rollback. The 25 retained matrix IDs have composed-boundary anchors; they are not 25 independent full-stack proofs.

Read the [current-state audit](docs/technical-spec/CURRENT_STATE_AUDIT.md) and [EVAL-01 case ledger](docs/technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) for commands, tested dirty-worktree scope, failures and assumptions. This documentation update does not rerun or extend that evidence.

Final execution relies on shared ERP fences for all configured policy/revocation writers, intact authority triggers and the PostgreSQL UTC clock. Deadline validation occurs at the deferred commit check, not at later WAL durability or network response. Manual publication recovery is not verified end-to-end; contention cost is unmeasured, and an earlier transient UNAVAILABLE remains an availability observation.

**VERIFIED V2 — bounded A/B/C comparison:** [EVAL-02](docs/technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md) records 11 selected scenarios × 3 variants with committed-state oracles. It also repairs deferred line flushing that falsely invalidated an unchanged approval across transactions; the fresh 75-post-test regression and all runners pass afterward. [EVAL-03](docs/technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md) and [EVAL-04](docs/technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) now report separated boundary measurements and the permitted claim wording. These are bounded results from the recorded dirty worktree, not a release or production performance guarantee. Historical evaluator numbers do not measure the current protected workflow.

**Not production-ready.** No general ERP/SAP compatibility, whole Procure-to-Pay coverage, instant/global revocation, distributed atomicity, HMAC non-repudiation, legal compliance or exactly-once external-effect claim. SAP is applicability discussion only.

## Start here

| Need | Source |
|---|---|
| Resume the next bounded task | [ACTIVE_TASK](ACTIVE_TASK.md) |
| Direction, title and RQs | [V2 master plan](docs/thesis-proposal/THESIS_V2_MASTER_PLAN.md) |
| Progress and remaining work | [V2 task board](docs/thesis-proposal/THESIS_V2_TASK_BOARD.md) |
| Active proposal | [Markdown proposal](docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md) |
| Chapters 3–5 working draft | [Thesis chapter draft](docs/thesis-proposal/THESIS_CHAPTERS_3_5_DRAFT.md) |
| Claims versus evidence | [Scope alignment](docs/thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) |
| All documentation | [Master index](docs/00_MASTER_INDEX.md) |
| Odoo setup, fences and recovery | [Addon README](custom_addons/pdp_authorizer/README.md) |
| CLI | [pectl guide](docs/cli/pectl.md) |

The proposal DOCX/PDF are derived artifacts of the Markdown source. Their current visual-QA status is recorded on the task board; artifact generation alone does not prove layout correctness.

## Reproduction

Use the addon setup/runbook and the exact commands in the ledger. `make test-odoo-e2e` is the isolated integration gate and recreates its test database; do not point it at a real ERP database. Run only the checks relevant to changed inputs. Documentation-only work does not require a Docker/Odoo rerun.

| Component | Path |
|---|---|
| Delegation/proof/capability | `internal/security/` |
| PDP transport and trust boundary | `internal/server/` |
| Policy storage and ERP fences | `internal/storage/` |
| Odoo enforcement and tests | `custom_addons/pdp_authorizer/` |
| Testbed and independent-session runners | `deployments/docker/` |
| Proposal artifact generator | `scripts/generate_master_thesis_proposal.py` |

## Graduation and ERP career direction

Build demonstrable SE skills in backend integration, authorization and database transactions through this bounded Odoo workflow. See the [SE-to-ERP roadmap](docs/thesis-proposal/SE_ERP_CAREER_ROADMAP.md). The [V1 archive](docs/thesis-proposal/archive/v1-2026-09-19/VERSION_INDEX.md) preserves the earlier PDP-performance framing; it is not the active proposal.
