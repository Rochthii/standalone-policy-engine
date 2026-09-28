---
name: erp-testing
description: Design, implement or assess bounded Odoo/PDP authorization tests and evidence for delegated purchase-order confirmation.
---

# ERP Authorization Testing — V2

Use this skill for Odoo/PDP proof, approval, transaction, replay, SoD, revocation or evidence work. Do not use it to claim universal ERP behavior, model/prompt safety, SAP compatibility or a performance win from unmatched benchmarks.

## Evidence rules

Read [`CURRENT_STATE_AUDIT.md`](../../../docs/technical-spec/CURRENT_STATE_AUDIT.md), [`EVALUATION_MATRIX.md`](../../../docs/technical-spec/EVALUATION_MATRIX.md) and `ACTIVE_TASK.md` first. The seven in-process delegation vectors and real Odoo mTLS cases are retained evidence with their documented boundaries. The suite verifies initial CBI/proof V2, AC v1 issue/verify/invalidation, approved final mutation, bounded two-session execution, injected rollback, lost-response retry and final-route outage. Independent-session concurrent business-field edits, external effects and cross-system policy-snapshot atomicity remain open.

- Each negative ERP case must assert the persistent outcome after the Odoo/PostgreSQL transaction: no unauthorized final business mutation, no false capability/command consumption.
- Test one real boundary when the change crosses it: Go unit/proof compatibility, generated gRPC client/server, Odoo ORM/PostgreSQL transaction, or the pinned testbed. Do not call an in-process vector E2E.
- Include a hard-deny case at the same high-impact amount as an approval-required case; prove that `DENY` plus an approval obligation cannot become a final effect or a consumed AC.
- Implement the named V2 cases in the matrix. Preserve `VERIFIED BASELINE`, `DESIGNED V2` and `VERIFIED V2` labels until evidence supports promotion.
- Test independent-human approval, agent/creator/delegator SoD, wrong role, cross tenant, tampering, expiry/revocation, key confusion, activity-only behavior, stale state, race, rollback, retry, bypass and outage when their owning task is implemented.

## Comparative and performance evidence

For RQ4, variants A (broad service account), B (policy only) and C (proposed mechanism) use the same fixture, action and environment. Measure evaluator, proof/capability verification, gRPC/mTLS, locked state reconstruction and ERP mutation separately; report raw p50/p95/p99/max, errors and environment metadata.

Historical 44,000x and fixed-nanosecond claims are retired. Do not compare an in-memory evaluator with an ERP transaction as a speedup. State the exact measured boundary and limitations.
