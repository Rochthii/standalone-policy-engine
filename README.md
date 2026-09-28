# Transaction-Bound Authorization for AI Agents in ERP

**Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

*Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17*

Research prototype for a Software Engineering graduation thesis, evaluated on one Odoo 17 `purchase.order` confirmation path.

**Author: Chăm Rốch Thi (PTIT)**

## Research focus

Can an AI agent commit only the exact ERP transaction a human delegated, while its authority and any required approval remain valid at commit time?

The agent is treated as a non-human software principal acting under bounded human delegation. The project studies authorization and transaction integrity—not model training, reasoning quality, orchestration or prompt filtering.

The contribution has three parts:

1. **Delegation-aware authorization:** who delegated to which agent, for what scope and validity period, under which business constraints.
2. **Transaction binding:** bind permission to the canonical business intent (CBI)—the exact action and authoritative ERP state—not merely to an agent or role.
3. **Commit-time enforcement:** revalidate under lock; bind human approval to the exact action; consume approval/command once with the ERP mutation.

The Go policy decision point (PDP), Odoo policy enforcement point (PEP), gRPC and mTLS implement the mechanism. PDP performance is supporting evidence, not the thesis contribution.

## Protected execution flow

```text
Human delegation → Agent tool proposal
  → Odoo PEP rebuilds CBI from authoritative ERP records
  → PDP: ALLOW or DENY
  → If ALLOW requires approval: persist "to approve"; release locks
  → Independent human approves the exact pending intent
  → Final execution: lock/reread, verify intent and current authority
  → Consume command/required approval + mutate PO in one ERP transaction
```

An ALLOW without an approval obligation follows the direct route. `REQUIRE_HUMAN_APPROVAL` qualifies ALLOW; it is not a third decision, and approval cannot override DENY. An Odoo Activity is only a notification. Amount, vendor and lines supplied by the agent are not authoritative facts.

## Evidence and limits

**Latest bounded gate (2026-09-28):** clean revision [`29afd70`](docs/technical-spec/evidence/CLEAN_BASELINE_GATE_2026_09_28.md) passed the fresh Odoo 17/mTLS/PDP/PostgreSQL testbed: **78 Odoo post-tests, zero failures/errors**, plus the isolated legacy-currency migration, mTLS probe, concurrency/retry, authority-ordering and 16 material-edit schedules. Earlier EVAL-01/02/03/04 evidence keeps its own recorded source revision and provenance. The 25 retained evaluation IDs are supported across composed boundaries; they are not 25 independent full-stack proofs.

See the [current-state audit](docs/technical-spec/CURRENT_STATE_AUDIT.md), [EVAL-01 case ledger](docs/technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md), [EVAL-02 comparison](docs/technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md), [EVAL-03 measurements](docs/technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md) and [EVAL-04 claim limits](docs/technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) for exact procedures, evidence and assumptions.

**Scope and limits:** this is a bounded research prototype, not production-ready. It covers one Odoo 17 PO-confirmation route—not all ERP workflows or SAP. Evidence assumes configured policy/revocation writers share the ERP fences and required database controls remain intact. Production load, end-to-end recovery, global instant revocation, distributed atomicity, legal compliance and exactly-once external effects are not established.

## Start here

| Need | Source |
|---|---|
| Current task checkpoint | [ACTIVE_TASK](ACTIVE_TASK.md) |
| Direction, title and RQs | [V2 master plan](docs/thesis-proposal/THESIS_V2_MASTER_PLAN.md) |
| Task status and evidence | [V2 task board](docs/thesis-proposal/THESIS_V2_TASK_BOARD.md) |
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
