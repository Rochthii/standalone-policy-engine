# Threat Model — Transaction-Bound AI-Agent Authorization V2

> **Status:** V2 design model, 2026-09-19.
> **Implementation truth:** [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md).
> **Scope:** One human-to-agent delegation hop and one Odoo 17 `purchase-order confirmation` path.

## 1. Security decision

The V2 mechanism must prevent an AI agent from committing a high-impact ERP mutation unless the authenticated caller, delegated authority, canonical business intent, material ERP state, current policy/revocation state and any required human approval are all valid at final execution.

This is a bounded transaction-integrity claim for the tested Odoo/PostgreSQL boundary. It is not a guarantee for arbitrary ERP systems, external side effects, SAP runtime integration, model safety or production deployment.

## 2. Actors and trust boundaries

| Actor or component | Trusted for | Not trusted for |
|---|---|---|
| Human delegator | Granting a bounded authority and requesting/revoking delegation. | Directly asserting agent identity, current ERP state or approval completion. |
| AI agent | Requesting a permitted tool call through the PEP. | Selecting its own scope, protected intent fields, approval or final business state. |
| Odoo PEP | Reading authoritative Odoo data, constructing request material and applying the controlled workflow transition. | Trusting client-supplied identity/amount/state without reconstruction. |
| Go PDP | Authenticating the caller, validating delegation/proof, evaluating policy/revocation and returning a decision/obligation. | Authoritatively reading mutable Odoo business state unless it is supplied through a defined trusted mechanism. |
| Odoo/PostgreSQL transaction | Persisting authorization attempt/approval state and the business mutation. | Guaranteeing exactly-once effects outside this database boundary. |
| Human approver | Approving or rejecting one bounded business intent when authorized by policy. | Expanding delegation scope, changing intent or bypassing SoD. |
| SAP | Future applicability target only. | Any current runtime, compatibility or effectiveness claim. |

## 3. Explicit assumptions

1. The Odoo PEP runs in the trusted ERP application boundary and obtains business fields from authoritative ORM records, not from agent prompts or raw client input.
2. JWT/mTLS and the existing versioned proof remain verified baseline controls for the stated Odoo/PDP boundary; HMAC establishes integrity for configured key holders, not non-repudiation.
3. The final protected mutation and authorization/approval consumption can share one Odoo/PostgreSQL transaction only for the selected workflow.
4. Human approval may take time. The system must not keep a row lock while waiting for a person; it must re-read and revalidate before the final action.
5. Dynamic HR status and daily-limit attributes are not trusted inputs in the current implementation and remain out of any V2 claim until implemented and tested.

## 4. Adversary model

The adversary can control an AI prompt, induce an agent to choose a tool/action, replay or alter a previously observed agent request, call a reachable API/entry point directly, race a mutable purchase order or approval, or use an unauthorized user as approver. The adversary cannot forge a valid mTLS client certificate, JWT signature, HMAC key or database transaction after the stated trust boundary has been reached.

Compromise of Odoo/PDP code, signing keys, PostgreSQL superuser access, host OS, administrator accounts or an authorized human approver is outside the technical guarantee. Those risks require operational controls beyond this thesis.

## 5. Threat-to-control matrix

| ID | Threat | Required V2 control | Current status | Required negative evidence |
|---|---|---|---|---|
| T1 | Caller or tenant spoofing | Mandatory JWT/mTLS, tenant binding and trusted signed principal attributes. | **VERIFIED BASELINE** | Missing/forged JWT and cross-tenant requests fail. |
| T2 | Delegation/proof tampering | Versioned full-tuple proof and active-grant/TTL/revocation validation. | **VERIFIED BASELINE** for implemented tuple | Altered protected tuple, expired proof and revoked grant fail. |
| T3 | Parameter substitution | [`CanonicalBusinessIntent v1`](./CANONICAL_BUSINESS_INTENT.md) binds action, record, tenant/company, amount in minor units, currency, vendor, line digest, state witness and command ID. | **VERIFIED V2** for the initial proof and pending-intent boundary; final revalidation remains planned | Every material-field modification invalidates proof/authorization. |
| T4 | Stale ERP state / TOCTOU | Material-state witness, final row lock/re-read and policy/revocation recheck immediately before mutation. | **PLANNED V2** | Changed PO state/version requires fresh authorization or approval. |
| T5 | Approval substitution or reuse | [`ApprovalCapability v1`](./APPROVAL_CAPABILITY.md) binds approval ID, intent hash, state witness, command/grant, authorized approver, expiry and one-time ID under a purpose-separated key. | **PARTIAL VERIFIED V2** for issue/verify, tamper, expiry, unknown-key and key-separation boundaries; invalidation/reuse at final execution remain planned | Wrong intent/approver, expiry, change, key confusion and reuse fail closed. |
| T6 | Delegated self-approval / SoD bypass | Evaluate creator/delegator/agent separation and current approver authority at capability issuance and final action. | **VERIFIED V2** at authenticated-human AC issuance; final current-authority recheck remains designed | Creator, delegator, agent, wrong-role and cross-tenant approver cases fail. |
| T7 | Replay, lost response or concurrent execution | Idempotent command ID and atomic consumption with business mutation. | **VERIFIED BASELINE** for nonce/retry path; **PLANNED V2** for final atomic approval/command semantics | Concurrent/retried command creates at most one committed effect. |
| T8 | Direct PEP/workflow bypass | Inventory and protect every final transition entry point in scope. | **PLANNED V2** | UI, API and supported approval routes cannot reach final mutation without PEP. |
| T9 | PDP outage or revocation-sync degradation | Fail closed for delegated high-impact actions; preserve only controlled non-final approval state where specified. | **VERIFIED BASELINE** for tested PDP outage and bounded revocation behavior | No unauthorized persistent mutation during outage/degraded state. |
| T10 | Prompt injection or hallucinated tool selection | Treat the model as untrusted; constrain its authority with T1-T9 controls. | **PARTIAL** | Out-of-scope tool/action/intent is rejected or requires valid human approval. |

## 6. Design invariants and test mapping

Named executable cases and persistent-state oracles are defined in [`EVALUATION_MATRIX.md`](./EVALUATION_MATRIX.md). `DESIGNED V2` is not implementation evidence.

| Threats | V2 invariant | Planned task |
|---|---|---|
| T1-T2 | Caller and delegation authority are authenticated, tenant-bound, valid and non-revoked. | `AUTH-P01`, `AUTH-N01`, `AUTH-N02`; V2-PROOF-01 to V2-PROOF-04. |
| T3-T4 | Every material intent field and current state witness are validated at final execution. CBI v1 defines the normative field/source contract. | `CBI-P01`, `CBI-N01`–`CBI-N05`, `TXN-N04`; V2-PROOF-01 to V2-PROOF-04, V2-TXN-02. |
| T5-T6 | Approval cannot expand authority and is bound to one authorized approver, intent and state. AC v1 defines the normative payload, SoD, lifecycle and key separation. | `APP-P01`, `APP-P02`, `APP-N01`–`APP-N05`; V2-APP-01 to V2-APP-04. |
| T7-T8 | One accepted command is consumed at most once with the scoped business mutation, and all final entry points enforce it. | `TXN-P01`, `TXN-P02`, `TXN-N01`–`TXN-N04`, `BOUND-N01`; V2-TXN-01 to V2-TXN-04. |
| T9-T10 | High-impact delegated action fails closed when authorization is unavailable; model output never overrides deterministic control. | `BOUND-N02`, `BOUND-N03`; V2-TXN-04 and V2-EVAL-01. |

## 7. Non-goals and evidence rule

Do not claim instant revocation, general prompt-injection prevention, dynamic HR/limit attenuation, non-repudiation, exactly-once external effects, SAP compatibility or production readiness from this model. Each V2 control becomes a thesis result only after its matching executable evidence passes.
