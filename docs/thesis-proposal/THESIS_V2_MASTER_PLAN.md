# Thesis V2 Master Plan

> **Status:** DIRECTION AND TITLE LOCKED — V2-DOC-05 SOURCE ALIGNMENT COMPLETE
> **Updated:** 2026-09-28
> **Task source:** [`THESIS_V2_TASK_BOARD.md`](./THESIS_V2_TASK_BOARD.md)
> **Evidence authority:** [`CURRENT_STATE_AUDIT.md`](../technical-spec/CURRENT_STATE_AUDIT.md)
> **Career alignment:** [`SE_ERP_CAREER_ROADMAP.md`](./SE_ERP_CAREER_ROADMAP.md)
> **V1 archive:** [`archive/v1-2026-09-19/VERSION_INDEX.md`](./archive/v1-2026-09-19/VERSION_INDEX.md)

> **Evidence checkpoint (2026-09-28):** EVAL-01 closed with 75 post-tests and separate runners; 25 retained IDs have composed-boundary anchors. [EVAL-02](../technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md) adds 33 bounded A/B/C outcomes and a CBI flush repair, followed by the passing fresh regression gate. [EVAL-03](../technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md) records bounded, separate boundary measurements; [EVAL-04](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) sets their claim limits. V2-WRITE-04 regenerated paired proposal exports from aligned Markdown; DOCX content parity and five-page PDF visual review passed. Per user scope decision, Word-native DOCX rendering is documented as unverified but is not a separate open gate. See the board and changelog for provenance and limits.

## 1. Outcome

Revise the thesis from a performance-centered PDP proposal into a bounded study of:

> **Transaction-bound authorization for delegated AI-agent actions in ERP.**

**Locked Vietnamese title:**

> **Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

**Locked English title:**

> **Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17**

The implementation target is one Odoo 17 purchase-order workflow. The title names the research focus and platform; the scope section specifies the single PO path. The Go PDP remains the inherited enforcement foundation, not the thesis novelty by itself.

The single long-term direction is **Software Engineering for secure ERP systems**: graduate with a production-shaped Odoo integration and use its transferable identity, authorization, workflow and transaction skills to move into broader ERP work, including SAP BTP/ABAP integration when a real SAP environment is available.

## 2. Central invariant

No high-impact ERP mutation may commit unless all conditions hold:

```text
authenticated trusted caller
+ valid 1-hop delegated authority
+ exact canonical business intent
+ current ERP material state matches
+ current policy/revocation check passes
+ required exact-action approval is valid
+ command and approval are consumed at most once
```

The strongest claim is limited to ERP state transitions inside the tested Odoo/PostgreSQL transaction boundary. External effects require outbox/downstream idempotency and are not exactly-once claims.

### Operational definition and decision flow

For this thesis, an **AI agent** is a non-human software principal that proposes business-tool calls on behalf of a human through bounded delegation. The thesis evaluates the authorization boundary, not model reasoning, model training, prompt filtering or agent orchestration. A small deterministic caller/tool harness may exercise the boundary; it must not be presented as an AI-model evaluation.

The Odoo PEP reconstructs canonical intent from authoritative ERP records; caller-supplied amount/vendor/line values are proposals, not trusted facts. The initial PDP decision is `ALLOW` or `DENY`; `REQUIRE_HUMAN_APPROVAL` is an obligation that may accompany `ALLOW`, never a third decision and never an override for `DENY`. At final execution, Odoo locks and rereads the in-scope state, reconstructs and checks the intent, revalidates authority/approval, then consumes the command and approval atomically with the Odoo/PostgreSQL mutation.

### Three contribution layers

1. **Delegation-aware authorization model:** human principal, agent identity, one-hop scope, validity and constraints.
2. **Transaction-bound authorization (core contribution):** canonical business intent, material-state witness, proof versioning, tamper and replay resistance.
3. **Commit-time enforcement:** locked revalidation, exact-action approval binding, one-time consumption and atomic in-scope ERP mutation.

Go PDP, Trie/DAG/evaluator, gRPC and mTLS are implementation mechanisms and inherited foundations. PDP/ERP performance is supporting evidence, not the primary contribution.

## 3. Research questions

| ID | Question |
|---|---|
| RQ1 | How can the system represent and verify a logical AI agent acting for a human delegator in one tenant under explicit scope, validity and business constraints? |
| RQ2 | How can delegated authorization be bound to canonical business intent and current ERP state at commit time to prevent tampering, replay and TOCTOU? |
| RQ3 | How can Separation of Duties and exact-action human approval be enforced without approval reuse, substitution or delegated self-approval? |
| RQ4 | Compared with broad service-account access and policy-only authorization, what security correctness and operational overhead does the proposed mechanism produce on the same Odoo workflow? |

RQ1–RQ4 are retained. During proposal alignment, keep “AI agent” scoped to the operational definition above and avoid implying that the thesis evaluates model safety or general AI governance.

## 4. Scope

### In scope

- One human-to-agent delegation hop.
- Odoo 17 purchase-order confirmation as the primary workflow.
- Canonical intent, material-state witness and proof versioning.
- Exact-action approval with expiry and one-time consumption.
- Commit-time revalidation, replay handling and transaction concurrency.
- Existing JWT, mTLS, policy, revocation and audit foundations.
- Functional, adversarial and boundary-separated performance evidence.
- A bounded architecture mapping from the verified Odoo design to SAP concepts; no SAP implementation claim.

### Out of scope

- Multi-hop or multi-agent delegation.
- Model training, prompt-filter research or orchestration.
- SAP integration, full P2P coverage or general ERP proof.
- OPA/Cedar performance competition.
- Edge deployment and external WORM archive work.
- Cross-organization non-repudiation and production certification.
- Nanosecond optimization unrelated to a failed acceptance gate.

## 5. Version policy

The previous proposal is preserved as the **V1 historical baseline** in [`archive/v1-2026-09-19/`](./archive/v1-2026-09-19/). Do not edit archived artifacts. V1 remains useful for provenance, supervisor comparison and documenting why the research direction changed.

From `V2-DOC-01` onward:

- `DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md` becomes the active V2 source.
- V1 is read-only historical material.
- Current-state audit and executable evidence remain authoritative over both proposal versions.
- Old benchmark claims may be discussed as superseded history, never as current results.

## 6. Platform decision

| Platform | Role in this work | Claim boundary |
|---|---|---|
| Odoo 17 | Primary implementation and empirical testbed. | Claims are limited to the tested purchase-order path and Odoo/PostgreSQL transaction boundary. |
| Go PDP | Reusable authorization and evidence-producing foundation. | Performance is supporting evidence, not the central novelty. |
| SAP | Post-graduation transfer target and thesis applicability discussion. | No SAP runtime, compatibility or effectiveness claim without a separate implementation and evaluation. |

## 7. Work phases and gates

| Phase | Outcome | Gate |
|---|---|---|
| 0. Direction lock | V1 is archived; V2 title, RQs, claims and non-goals agree. | V1 is preserved and only V2 is active. |
| 1. Security model | Threat model, canonical intent, approval model and invariants are specified. | Every protected field has an authoritative source and a negative test requirement. |
| 2. Intent proof v2 | Go and Python protect the same material intent. | Golden vector and tamper matrix pass; high-impact path cannot downgrade to v1. |
| 3. Exact-action approval | Approval is bound to one intent and authorized approver. | Changed intent, wrong approver, expiry and reuse all fail closed. |
| 4. Commit enforcement | Final revalidation and mutation share one transaction boundary. | Concurrent/retried execution produces at most one committed effect. |
| 5. Evaluation | Variants A/B/C run the same bounded workload. | Raw functional, security and performance evidence is reproducible. |
| 6. Thesis freeze | Chapters and proposal match verified evidence. | Claim-to-evidence audit passes; DOCX/PDF render correctly. |

## 8. Execution rules

To minimize context and token use:

1. Execute one dependency-complete task from the task board per turn; batch related acceptance gaps without nested task IDs.
2. Read only the files listed by that task and their directly called code.
3. Start with the narrow validation listed for the task.
4. Expand validation only when the change crosses a real boundary.
5. Update the task row with status and evidence before selecting the next task.
6. Do not rewrite frozen evidence; add new evidence for changed behavior.
7. Do not update `CURRENT_STATE_AUDIT.md` until executable evidence passes.
8. Do not regenerate DOCX/PDF until Markdown and the generator contain the same v2 claims.

## 9. Claim discipline

### Allowed after corresponding evidence passes

- Transaction-bound for the tested Odoo purchase-order path.
- Exact-action approval for the fields included in the canonical schema.
- At-most-once committed ERP effect for one command ID inside the tested database boundary.
- Bounded revocation behavior for the measured replica/test configuration.

### Never infer automatically

- EU AI Act compliance.
- Production readiness.
- Instant revocation.
- Exactly-once external side effects.
- General prompt-injection prevention.
- General ERP or multi-agent security.
- Non-repudiation from shared-key HMAC.

## 10. Completion definition

The v2 direction is complete only when:

- Proposal, generator, chapter mapping and evidence alignment use the same title and RQs.
- Currency, vendor/payee, action, resource and material record state are bound to intent.
- A changed material field invalidates prior approval.
- Final execution re-evaluates current authority and state.
- Approval and command consumption are atomic with the business mutation.
- The relevant Odoo regression suite passes on the tested final inputs; use the ledger for the current count rather than freezing the historical seven-case baseline.
- New approval, tamper, policy/revocation and concurrency cases pass on real Odoo/PostgreSQL.
- RQ4 reports evaluator, authorization and ERP mutation boundaries separately.
