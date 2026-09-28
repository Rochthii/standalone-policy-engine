# Project Documentation Index

**Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

*Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17*

Updated 2026-09-28. Active direction: delegation → authoritative intent binding → commit enforcement → bounded evaluation on one Odoo 17 purchase-order confirmation path. Go PDP performance is supporting evidence; SAP is applicability discussion only.

## Read by purpose

| Purpose | Authoritative entry |
|---|---|
| Resume work without rescanning the repository | [Active task](../ACTIVE_TASK.md) |
| Locked title, agent definition, three contributions and RQ1–RQ4 | [V2 master plan](thesis-proposal/THESIS_V2_MASTER_PLAN.md) |
| Task status and remaining sequence | [V2 task board](thesis-proposal/THESIS_V2_TASK_BOARD.md) |
| Active thesis proposal and artifact source | [Proposal Markdown](thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md) |
| Working draft of Chapters 3–5 | [Chapter draft](thesis-proposal/THESIS_CHAPTERS_3_5_DRAFT.md) |
| Scope, evidence and limits | [Alignment](thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) |
| Chapter ownership and evidence mapping | [Chapter map](technical-spec/THESIS_CHAPTER_MAPPING.md) |
| SE-to-ERP learning and portfolio scope | [Career roadmap](thesis-proposal/SE_ERP_CAREER_ROADMAP.md) |
| Implementation truth and open release gates | [Current audit](technical-spec/CURRENT_STATE_AUDIT.md), [readiness](technical-spec/PRODUCTION_READINESS_CHECKLIST.md) |

## Security contracts and executable evidence

| Boundary | Source |
|---|---|
| Attacker, trust and assumptions | [Threat model](technical-spec/THREAT_MODEL.md) |
| Protected mutation invariants | [Security invariants](technical-spec/SECURITY_INVARIANTS.md) |
| Canonical fields, exact minor-unit money and state witness | [Canonical intent](technical-spec/CANONICAL_BUSINESS_INTENT.md) |
| Exact-action approval, key separation and SoD | [Approval capability](technical-spec/APPROVAL_CAPABILITY.md) |
| Case IDs and persistent-state oracles | [Evaluation matrix](technical-spec/EVALUATION_MATRIX.md) |
| EVAL-01 closure, commands, failures and precise limits | [Case ledger](technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) |
| Odoo configuration, shared fences and recovery | [Addon README](../custom_addons/pdp_authorizer/README.md) |
| Client protocol and integration | [Protocol contract](technical-spec/PROTOCOL_CONTRACT.md), [Odoo PEP](technical-spec/PEP_ODOO_INTEGRATION.md) |

The EVAL-01 closure records 75 post-tests with zero failures/errors plus independent-session runners. The 25 retained IDs have composed evidence, not universal ERP proofs. [EVAL-02](technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md) adds 33 bounded A/B/C outcomes and a verified deferred-line-flush repair. [EVAL-03](technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md) records separate Go and Odoo boundary distributions; [EVAL-04](technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) limits their interpretation. Final authority depends on configured shared writers, intact triggers and the database clock; deferred expiry validation does not cover a later WAL/network instant. This remains a research prototype.

## Supporting implementation and operations

- [Architecture](technical-spec/ARCH_SPEC.md), [DSL](technical-spec/POLICY_DSL_SPEC.md), [agent tooling](technical-spec/AGENT_TOOLING_SPEC.md).
- [Replay/idempotency](technical-spec/DELEGATION_REPLAY_IDEMPOTENCY.md), [operations](technical-spec/RUNBOOK_OPS.md), [benchmark reproduction](technical-spec/BENCHMARK_REPRODUCIBILITY.md).
- [CLI](cli/pectl.md), [skill catalog audit](technical-spec/SKILL_CATALOG_AUDIT.md), [agent guide](../AGENTS.md), [changelog](../CHANGELOG.md).
- [Artifact generator](../scripts/generate_master_thesis_proposal.py) exports the active Markdown to DOCX/PDF. Rendering status is on the task board.

For implementation claims, current code/protocol/executable tests outrank the audit, checkpoint/board, designs and history. Supporting documents do not override those sources.

## Historical material

[V1 proposal archive](thesis-proposal/archive/v1-2026-09-19/VERSION_INDEX.md) is read-only. Historical benchmarks and the earlier [implementation board](technical-spec/IMPLEMENTATION_TASK_BOARD.md) retain provenance, not a new V2 completion or production-readiness claim.
