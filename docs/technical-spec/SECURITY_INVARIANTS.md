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

This invariant is **PLANNED V2**. Existing behavior is a security baseline, not proof that every condition above currently holds.

## 2. I1 — Delegation authority integrity

The executed agent may act only for the authenticated delegator, tenant, scope and validity window established by one active delegation grant. The agent cannot substitute its own delegator, tenant or grant.

**Current status: VERIFIED BASELINE.** Mandatory tenant-bound JWT, mTLS at the Odoo/PDP boundary, versioned full-tuple HMAC proof, TTL, active-grant checks and revocation are covered within their recorded test boundaries. HMAC is integrity/authenticity for configured key holders, not non-repudiation.

**Negative cases:** missing/forged JWT, cross-tenant call, altered protected proof, expired proof and revoked grant fail closed.

**Evaluation mapping:** positive `AUTH-P01`; negative `AUTH-N01`, `AUTH-N02` in [`EVALUATION_MATRIX.md`](./EVALUATION_MATRIX.md).

## 3. I2 — Intent and state binding

Authorization for a high-impact action is valid only for one `CanonicalBusinessIntent` and one material-state witness. At minimum, the schema must define action, resource, command ID, amount in currency minor units, currency, vendor/payee, line digest, tenant, delegator, agent, policy/proof version and record state/version.

The Odoo PEP must reconstruct the intent from authoritative ORM records and compare it with the protected value immediately before final execution. A changed material field or state witness requires new authorization and, if required, new approval.

**Current status: PARTIAL V2.** Go/Python unit tests agree on canonical multi-line bytes/digest, state witness, CBI bytes/hash and V2 proof for one shared fixture; exact-money rejection and material-field tampering are covered in the pure protocol boundary. Trusted ORM reconstruction, protected-route enforcement and commit-time locked revalidation remain planned; current route evidence does not yet establish I2.

**Negative cases:** changed amount, currency, vendor, line digest, action, resource or record version fails before mutation.

**Evaluation mapping:** positive/interoperability `CBI-P01`; negative `CBI-N01`–`CBI-N05` and final race `TXN-N04`.

## 4. I3 — Exact-action approval and Separation of Duties

An [`ApprovalCapability v1`](./APPROVAL_CAPABILITY.md) is valid only for one approval ID, authorized approver, intent hash, state witness, command/grant, expiry and one-time ID under a purpose-separated approval key. It cannot expand a delegator's scope, be reused for another intent, be verified as a delegation proof, or be exercised by a creator, delegator or agent under the thesis SoD policy.

**Current status: PARTIAL VERIFIED V2 through AC v1 issuance.** The tested Odoo route persists `to approve`, one exact pending CBI and one Activity without rollback. The issuance operation derives the human from `env.user`, locks/reconstructs the unchanged intent, enforces active internal purchase-manager role, same tenant/company, creator/delegator/agent separation and a current live-PDP approval decision, then persists one PDP-issued/verified purpose-separated capability and `approved` state. Tamper, expiry, unknown/key-confused credentials and invalid issuer identity fail closed in focused tests. Invalidation and atomic one-time consumption remain **NOT IMPLEMENTED**.

**Negative cases:** wrong approver/role, creator or delegator self-approval, expired approval, changed intent/state and consumed approval fail closed.

**Evaluation mapping:** positive `APP-P01`, `APP-P02`; negative `APP-N01`–`APP-N05`.

## 5. I4 — Commit-time revalidation and at-most-once scoped effect

The final transition re-locks and re-reads the business record, validates I1-I3 plus current policy/revocation, then atomically consumes the command/approval and applies the in-scope ERP mutation in one Odoo/PostgreSQL transaction. No lock is held while awaiting human review.

**Current status: PLANNED V2.** Existing nonce ledger and two-session retry evidence demonstrate bounded replay behavior, not the final approval/command atomicity required here.

**Negative cases:** concurrent execution, lost-response retry, state change during approval and PDP outage leave no unauthorized persistent mutation; one command yields at most one committed purchase-order effect.

**Evaluation mapping:** positive `TXN-P01`, `TXN-P02`; negative `TXN-N01`–`TXN-N04`.

## 6. I5 — Fail-closed authorization boundary

Missing identity, invalid proof, unavailable/degraded delegated authorization, unsupported canonical field or uncertain approval state denies final high-impact execution. A controlled approval route may persist only a non-final review state, never the protected business effect.

**Current status: VERIFIED BASELINE for tested proof, outage and non-rollback approval paths; PLANNED V2 for final execution semantics.**

**Evaluation mapping:** controlled non-final route `APP-P01`; negative boundary cases `BOUND-N01`–`BOUND-N03`.

## 7. Limits

These invariants do not establish dynamic HR/daily-limit attenuation, instant revocation, exactly-once external side effects, general prompt-injection prevention, general ERP validity, SAP compatibility, production readiness or regulatory compliance. Each implementation claim requires its mapped executable evidence in `THESIS_V2_TASK_BOARD.md`.
