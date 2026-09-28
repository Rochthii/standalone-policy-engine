---
name: agent-authorization
description: Design or implement one-hop delegated AI-agent authorization, canonical intent, approval capability, SoD, proof and revocation for the bounded Odoo ERP workflow.
---

# Agent Authorization — V2

Use this skill only for delegated identity, proof, canonical business intent, exact-action approval, revocation or SoD work. Do not use it for model safety, general IAM, benchmark tuning or multi-agent design.

## Authority and status

Read [`CURRENT_STATE_AUDIT.md`](../../../docs/technical-spec/CURRENT_STATE_AUDIT.md), then the active task and the directly relevant V2 schema. CBI/proof V2, mTLS/JWT, bounded revocation, AC v1 issue/verify and the non-rollback approval route have bounded evidence. One approved Odoo final route revalidates exact intent/current authority and consumes approval/command with the PO mutation in one transaction. V2-TXN-04 verifies bounded two-session execution, injected rollback, lost-response retry and final-route outage; concurrent business-field edits, external effects and cross-system policy-snapshot atomicity remain unverified.

## Non-negotiable rules

- Authenticate the caller and bind tenant, human delegator, one agent and active grant. Treat signed identity attributes as authoritative; never trust request-body identity.
- Construct protected business fields from locked Odoo/PostgreSQL records. Bind the V2 proof to the CBI hash, grant, agent, validity and key metadata; reject unknown/downgraded versions.
- Use deterministic, length-delimited canonical bytes. Authorization money is integer minor units; reject floats, malformed decimal/currency values and ambiguous encoding.
- Recheck current policy and revocation before final mutation. Durable revocation has bounded recorded evidence; do not claim instant or universal propagation.
- When approval is required, bind capability to one pending intent hash/state witness, independent authorized approver, expiry and one-time ID. Activity notification is not approval evidence.
- Treat a human-approval obligation as satisfiable only on a current PDP `ALLOW`; an AC must never turn `DENY` into permission, even when `DENY` includes that obligation.
- Enforce SoD for agent, creator, delegator, wrong-role and cross-tenant approvers at issuance and final execution.
- Keep approval and delegation key rings/domain separators separate. HMAC provides configured-key-holder integrity, not non-repudiation.
- Consume command/approval atomically with the in-scope mutation. No lock is held while waiting for a human; final execution locks and re-reads state.

## Validation

Map changes to named cases in [`EVALUATION_MATRIX.md`](../../../docs/technical-spec/EVALUATION_MATRIX.md). Start with focused Go/Python/Odoo tests named by `ACTIVE_TASK.md`; negative ERP cases must prove no unauthorized persistent business mutation. Record new implementation claims only after boundary evidence passes.
