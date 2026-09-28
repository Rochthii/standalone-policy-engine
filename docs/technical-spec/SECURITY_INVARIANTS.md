# Security Invariants — Transaction-Bound Authorization V2

> **Status:** V2 design invariants, 2026-09-19.
> **Evidence authority:** [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md).
> **Threat source:** [`THREAT_MODEL.md`](./THREAT_MODEL.md).

## 1. Central invariant

For the tested Odoo 17 purchase-order confirmation path, no high-impact ERP mutation may commit unless all of the following hold:

```text
authenticated trusted caller
+ valid one-hop delegated authority
+ exact canonical business intent
+ current material ERP state matches
+ current policy and revocation check passes
+ required exact-action approval is valid
+ command and approval are consumed at most once
```

This invariant is **PARTIAL VERIFIED V2** for one bounded approved Odoo final route, including two-session execution, rollback, retry and outage checks. Concurrent business-field edits, ambiguous-state cases and cross-system policy-snapshot atomicity still prevent a full invariant claim.

## 2. I1 — Delegation authority integrity

The executed agent may act only for the authenticated delegator, tenant, scope and validity window established by one active delegation grant. The agent cannot substitute its own delegator, tenant or grant.

**Current status: VERIFIED BASELINE.** Mandatory tenant-bound JWT, mTLS at the Odoo/PDP boundary, versioned full-tuple HMAC proof, TTL, active-grant checks and revocation are covered within their recorded test boundaries. HMAC is integrity/authenticity for configured key holders, not non-repudiation.

**Negative cases:** missing/forged JWT, cross-tenant call, altered protected proof, expired proof and revoked grant fail closed.

**Evaluation mapping:** positive `AUTH-P01`; negative `AUTH-N01`, `AUTH-N02` in [`EVALUATION_MATRIX.md`](./EVALUATION_MATRIX.md).

## 3. I2 — Intent and state binding

Authorization for a high-impact action is valid only for one `CanonicalBusinessIntent` and one material-state witness. At minimum, the schema must define action, resource, command ID, amount in currency minor units, currency, vendor/payee, line digest, tenant, delegator, agent, policy/proof version and record state/version.

The Odoo PEP must reconstruct the intent from authoritative ORM records and compare it with the protected value immediately before final execution. A changed material field or state witness requires new authorization and, if required, new approval.

**Current status: VERIFIED V2 FOR ENUMERATED ORM PATHS.** Go/Python vectors agree on canonical bytes/digest, witness, CBI hash and V2 proof. Exact one-minor-unit ERP changes invalidate old approval. A reproduced line-membership phantom is repaired by parent locking/version updates before protected line CRUD. Sixteen independent-session schedules cover description/price, insert/delete, tax relation, vendor/currency/state in both orderings with fresh commit oracles. Other scalar line fields use the same guarded write path and sequential tamper evidence. This is not arbitrary SQL/extension coverage, a proof for cross-order moves or a full Cartesian race matrix; see the EVAL-01 ledger. The separately bounded I4 authority-ordering evidence and time semantics are recorded below.

**Negative cases:** changed amount, currency, vendor, line digest, action, resource or record version fails before mutation.

**Evaluation mapping:** positive/interoperability `CBI-P01`; negative `CBI-N01`–`CBI-N05` and final race `TXN-N04`.

## 4. I3 — Exact-action approval and Separation of Duties

An [`ApprovalCapability v1`](./APPROVAL_CAPABILITY.md) is valid only for one approval ID, authorized approver, intent hash, state witness, command/grant, expiry and one-time ID under a purpose-separated approval key. It cannot expand a delegator's scope, be reused for another intent, be verified as a delegation proof, or be exercised by a creator, delegator or agent under the thesis SoD policy.

**Current status: PARTIAL VERIFIED V2 through bounded final execution.** The tested Odoo route persists `to approve`, one exact pending CBI and one Activity. Authenticated-human issuance enforces role, tenant/company and SoD before persisting a PDP-issued AC. The public approved final route revalidates AC/current authority and consumes it in the PO mutation transaction; tampered/expired AC and stale authority leave the PO non-final. Two-session consumption, rollback and retry pass for one PO; broader state-edit races remain open.

**Negative cases:** wrong approver/role, creator or delegator self-approval, expired approval, changed intent/state and consumed approval fail closed.

**Evaluation mapping:** positive `APP-P01`, `APP-P02`; negative `APP-N01`–`APP-N05`.

## 5. I4 — Commit-time revalidation and at-most-once scoped effect

**Authority ordering update (2026-09-27):** Protected initial-ALLOW and approved-final transactions hold grant, local-authority epoch and policy-revision locks through commit. Configured policy writers commit an unavailable barrier before touching PDP storage and publish its committed revision afterward; failure leaves it unavailable. Both evaluated approver/agent revisions must match the locked ready row. Local user/group/company/grant writes advance the epoch, including membership relation changes. Three grant schedules, four after-ALLOW policy/role schedules and two real-clock expiry rollback cases pass; see the [case ledger](./evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md). Every external authority writer must share the configured ERP fence. Validity is checked at PostgreSQL deferred commit validation using the database clock, not at a later WAL flush or response instant. No distributed transaction or unconfigured-writer guarantee is asserted.

The final transition re-locks and re-reads the business record, validates I1-I3 plus current policy/revocation, then atomically consumes the command/approval and applies the in-scope ERP mutation in one Odoo/PostgreSQL transaction. No lock is held while awaiting human review.

A current PDP `DENY` is final even if it carries `REQUIRE_HUMAN_APPROVAL`; only an `ALLOW` with that obligation can be satisfied by AC v1. Otherwise an unrelated hard forbid could be bypassed.

**Current status: VERIFIED V2 FOR THE CONFIGURED BOUNDED ROUTE.** The approved final route checks locked CBI/current authority and writes approval `consumed`, PO final state and command `executed` in one Odoo/PostgreSQL transaction. Approved two-session execution, rollback, retry, outage, 16 enumerated material schedules and the authority schedules above pass in the fresh EVAL-01 gate. Atomic consumption is local to ERP; cross-database authority ordering relies on the publication barrier, not a shared snapshot. All prior SQL/extension, recovery, clock and external-effect exclusions remain.

**Negative cases:** concurrent execution, lost-response retry, state change during approval and PDP outage leave no unauthorized persistent mutation; one command yields at most one committed purchase-order effect.

**Evaluation mapping:** positive `TXN-P01`, `TXN-P02`; negative `TXN-N01`–`TXN-N04`.

## 6. I5 — Fail-closed authorization boundary

Missing identity, invalid proof, unavailable/degraded delegated authorization, unsupported canonical field or uncertain approval state denies final high-impact execution. A controlled approval route may persist only a non-final review state, never the protected business effect.

**Current status: PARTIAL VERIFIED V2** for initial proof/outage and bounded approved-final execution, including AC-verifier transport and late agent-decision outages with retry. Ambiguous-state coverage remains open.

**Evaluation mapping:** controlled non-final route `APP-P01`; negative boundary cases `BOUND-N01`–`BOUND-N03`.

## 7. Limits

These invariants do not establish dynamic HR/daily-limit attenuation, instant revocation, exactly-once external side effects, general prompt-injection prevention, general ERP validity, SAP compatibility, production readiness or regulatory compliance. Each implementation claim requires its mapped executable evidence in `THESIS_V2_TASK_BOARD.md`.
