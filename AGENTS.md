# Standalone Policy Engine — Agent Guide

## Authority and claim discipline

For implementation facts, use this order of authority:

1. current source, protocol and executable tests;
2. [`docs/technical-spec/CURRENT_STATE_AUDIT.md`](docs/technical-spec/CURRENT_STATE_AUDIT.md);
3. [`ACTIVE_TASK.md`](ACTIVE_TASK.md) and [`docs/thesis-proposal/THESIS_V2_TASK_BOARD.md`](docs/thesis-proposal/THESIS_V2_TASK_BOARD.md);
4. V2 design documents under `docs/technical-spec/`;
5. historical proposal, benchmark and changelog entries.

If sources disagree, do not infer a stronger claim. Label the conflict and follow the higher authority. The repository is **not production ready**; do not claim general ERP security, SAP compatibility, legal non-repudiation, EU AI Act compliance, instant revocation or exactly-once external effects.

## Active thesis direction

The active contribution is **delegation-aware, transaction-bound authorization for high-impact AI-agent actions in ERP**, evaluated only on one Odoo 17 `purchase.order` confirmation path.

The V2 central invariant is: a protected mutation may commit only after trusted caller identity, valid one-hop delegated authority, exact canonical intent, current ERP state, current policy/revocation, required exact-action approval and atomic one-time consumption all pass.

- [`CANONICAL_BUSINESS_INTENT.md`](docs/technical-spec/CANONICAL_BUSINESS_INTENT.md) and [`APPROVAL_CAPABILITY.md`](docs/technical-spec/APPROVAL_CAPABILITY.md) remain normative contracts; the initial CBI/proof V2 and AC v1 issue/verify boundary is implemented and evidenced, while final transaction consumption remains open.
- The current implementation uses CBI/proof V2 for the high-impact Odoo route, a purpose-separated AC v1 issuance/verification boundary and a non-rollback `to approve` Activity route. None of these proves final commit-time revalidation or atomic business mutation.
- Odoo 17 is the only implementation/evaluation platform. SAP is an applicability discussion only.
- The V1 proposal is read-only historical material in `docs/thesis-proposal/archive/`.

## Work policy

1. Execute one dependency-complete task from `ACTIVE_TASK.md` per turn.
2. Read only files named by that task and directly needed code. Preserve unrelated worktree changes.
3. Keep design, implementation and evidence labels distinct: `VERIFIED BASELINE`, `DESIGNED V2`, `PLANNED V2` and `VERIFIED V2` are not interchangeable.
4. Do not update `CURRENT_STATE_AUDIT.md` merely because a design document or unit test exists; add a claim only after the stated boundary evidence passes.
5. Update the task board, active task and changelog when a task materially changes scope, agent guidance or evidence interpretation.
6. Preserve V1 compatibility until the task board explicitly authorizes a compatibility change. Never silently downgrade a protected V2 path to V1.

## V2 security rules

- Reconstruct protected business fields from locked Odoo/PostgreSQL records; never trust prompt text, tool arguments or caller context for them.
- Money is exact integer minor units at the authorization boundary. Do not sign, compare or canonicalize floating-point money.
- A human approval binds one pending intent hash and state witness. An Odoo Activity is notification only, not approval evidence.
- Enforce SoD for agent, creator, delegator, wrong-role and cross-tenant approvers at issuance and final execution.
- Use purpose-separated approval and delegation key domains. HMAC provides integrity for configured key holders, not non-repudiation.
- Final authorization re-locks/re-reads state and consumes command/approval atomically with the in-scope Odoo/PostgreSQL mutation. Never retain a row lock while waiting for a human.

## Skill routing

Read [`docs/technical-spec/SKILL_CATALOG_AUDIT.md`](docs/technical-spec/SKILL_CATALOG_AUDIT.md) before relying on any repository-local skill for implementation claims.

- Use `agent-authorization` for V2 identity, proof, canonical intent, approval capability, revocation and SoD work.
- Use `erp-testing` for Odoo/PDP authorization tests and evidence matrices.
- Use the subsystem skill only when changing that subsystem; do not preload unrelated skills.
- `grpc-dataplane`, `storage-audit`, `engine-evaluator` and other catalog entries remain subject to their remediation status in the audit.

## Validation and documentation

- Start with the focused validation named in `ACTIVE_TASK.md`; expand only when a real boundary changes.
- Every V2 negative ERP case must assert no unauthorized persistent business mutation, not merely an exception.
- Keep evaluator, proof/capability, gRPC/mTLS, locked state reconstruction and ERP mutation measurements separate.
- Historical performance values, especially 44,000x comparisons and fixed nanosecond claims, are retired unless the current audit/evidence explicitly supports the exact boundary.
- [`CHANGELOG.md`](CHANGELOG.md) is chronological history. Add superseding entries; do not rewrite old entries as current evidence.

## Key paths

| Purpose | Path |
|---|---|
| Active task | `ACTIVE_TASK.md` |
| V2 plan/board | `docs/thesis-proposal/THESIS_V2_MASTER_PLAN.md`, `docs/thesis-proposal/THESIS_V2_TASK_BOARD.md` |
| Current implementation truth | `docs/technical-spec/CURRENT_STATE_AUDIT.md` |
| Threats/invariants/evaluation | `docs/technical-spec/THREAT_MODEL.md`, `docs/technical-spec/SECURITY_INVARIANTS.md`, `docs/technical-spec/EVALUATION_MATRIX.md` |
| Odoo PEP | `custom_addons/pdp_authorizer/` |
| Go security code | `internal/security/` |
