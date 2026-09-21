# Evaluation Matrix — Transaction-Bound Authorization V2

> **Status:** V2 implementation and evidence matrix, updated 2026-09-21.
> **Evidence authority:** [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md).
> **Normative schemas:** [`CANONICAL_BUSINESS_INTENT.md`](./CANONICAL_BUSINESS_INTENT.md) and [`APPROVAL_CAPABILITY.md`](./APPROVAL_CAPABILITY.md).

This matrix separates existing baseline evidence from cases required to establish the V2 thesis claims. A designed case is not a passed test. Only executable results recorded against a source revision may move a V2 case to `VERIFIED`.

## 1. Status and outcome rules

| Label | Meaning |
|---|---|
| `VERIFIED BASELINE` | Existing bounded behavior has executable evidence but does not by itself satisfy the V2 invariant. |
| `DESIGNED V2` | Inputs, oracle and persistent-state outcome are specified; implementation/evidence do not yet exist. |
| `VERIFIED V2` | Reserved for a future case whose required real-boundary evidence passes and is recorded. |

For every negative Odoo case, success means **no unauthorized persistent business mutation**. An expected denial may persist only an explicitly allowed non-final approval/audit state. A raised exception alone is insufficient evidence: the test must query PostgreSQL/Odoo state after the transaction boundary.

## 2. Invariant traceability

| Invariant | Normative requirement | Positive cases | Negative cases | Implementation tasks |
|---|---|---|---|---|
| **I1 — Delegation authority integrity** | Authenticated tenant-bound one-hop grant, valid proof/TTL and current revocation state. | `AUTH-P01` | `AUTH-N01`, `AUTH-N02` | V2-PROOF-01 to V2-PROOF-04; regression through V2-TXN-04. |
| **I2 — Intent and state binding** | CBI v1 exact action/record, minor-unit money, currency, vendor, line digest, state witness and command identity. | `CBI-P01` | `CBI-N01` to `CBI-N05`, `TXN-N04` | V2-PROOF-01 to V2-PROOF-04; V2-TXN-02. |
| **I3 — Exact-action approval and SoD** | AC v1 binds one independent authorized human, intent/state, expiry, one-time ID and purpose-separated key. | `APP-P01`, `APP-P02` | `APP-N01` to `APP-N05` | V2-APP-01 to V2-APP-04. |
| **I4 — Commit-time revalidation and at-most-once scoped effect** | Current checks plus atomic command/capability consumption and mutation. | `TXN-P01`, `TXN-P02` | `TXN-N01` to `TXN-N04` | V2-TXN-01 to V2-TXN-04. |
| **I5 — Fail-closed authorization boundary** | Every final route rejects unavailable, ambiguous, unsupported or bypassed authorization. | `APP-P01` | `BOUND-N01` to `BOUND-N03` | V2-PROOF-04; V2-TXN-01; V2-TXN-04. |

## 3. V2 executable case specification

### 3.1 Delegation authority and canonical intent

| ID | Type | Setup/action | Required oracle | Expected persistent outcome | Status |
|---|---|---|---|---|---|
| `AUTH-P01` | Positive | Authenticated agent presents an active same-tenant one-hop grant and valid V2 proof for unchanged CBI. | Proof, tenant, grant, TTL and revocation checks pass; policy evaluation is reached. | No mutation occurs from verification alone; later workflow outcome follows the policy result. | `VERIFIED V2` for the initial Odoo/mTLS route; final approval path remains open. |
| `AUTH-N01` | Negative | Missing/forged JWT, agent mismatch or cross-tenant/company request. | Rejected before business mutation and before trusting caller context. | PO, command and approval rows remain unchanged. | `VERIFIED BASELINE` for listed JWT/tenant paths; V2 regression required. |
| `AUTH-N02` | Negative | Proof/grant is expired, revoked, inactive, malformed or substituted. | Fail closed before final transition. | No final PO mutation; no capability consumption. | `VERIFIED V2` for malformed/substituted and live revoked initial route; approval-capability regression remains open. |
| `CBI-P01` | Positive/interoperability | Go and Python serialize the same persisted PO fixture, including non-integer currency amount and multiple lines. | Canonical bytes, line digest, state witness and intent hash are byte-identical. | Pure serialization case; database fixture remains unchanged. | `VERIFIED V2` by shared multi-line vector plus exact persisted-money Odoo reconstruction. |
| `CBI-N01` | Negative | Change `amount_minor` by one unit or present float/exponent/non-integral money. | Old proof/approval fails; malformed money is rejected before signing. | No authorization, approval or final PO mutation. | `VERIFIED V2` at proof/parser boundary; persistent-mutation oracle remains for V2-EVAL-01. |
| `CBI-N02` | Negative tamper matrix | Independently change tenant/company, action, resource, vendor or currency while reusing old proof. | Every subcase changes intent hash and fails verification. | No command consumption or final PO mutation. | `DESIGNED V2` |
| `CBI-N03` | Negative tamper matrix | Add/remove/reorder a line or change product, description, UoM, quantity, unit price, tax, planned date or line write version. | Line digest and intent hash change; old proof/approval fails. | Changed draft may persist only if separately authorized; protected confirmation does not occur. | `VERIFIED V2` at canonical/proof boundary; persistent-mutation oracle remains for V2-EVAL-01. |
| `CBI-N04` | Negative stale state | Change PO state or parent write version after intent/approval creation. | Reconstructed state witness differs and requires a new intent/approval. | Existing approval becomes unusable; no final mutation from it. | `VERIFIED V2` at proof boundary; post-approval invalidation remains V2-APP-04/V2-TXN-02. |
| `CBI-N05` | Negative protocol | Use unknown schema/material extension, malformed canonical encoding or attempt V1 downgrade on the protected route. | Fail closed with no algorithm/version fallback. | PO remains non-final; command/approval remain unconsumed. | `VERIFIED V2` for unknown field/encoding and V1 downgrade before policy evaluation; extension inventory remains V2-TXN-01. |

### 3.2 Pending intent, human approval and key separation

| ID | Type | Setup/action | Required oracle | Expected persistent outcome | Status |
|---|---|---|---|---|---|
| `APP-P01` | Positive non-final | High-impact agent request receives approval obligation. | PEP writes `to approve`, then reconstructs and persists exactly one post-transition pending CBI and one Activity. | PO is non-final; one pending approval record exists; no AC v1 and no protected effect yet. | `VERIFIED V2` for the fresh-database Odoo/PostgreSQL/mTLS path; identical retry produces no duplicate. |
| `APP-P02` | Positive issuance | Independent same-tenant/company human with current permission approves unchanged pending CBI. | PDP-side authority issues one valid AC v1 under the approval key; no final mutation occurs at issuance. | Approval row becomes `approved`; PO remains `to approve`; capability/one-time ID are unique. | `VERIFIED V2` for typed PDP issue/verify plus the fresh-database Odoo/PostgreSQL/mTLS transition and idempotent retry. |
| `APP-N01` | Negative SoD | Agent, creator or delegator attempts to approve. | Subject equality/agent-type check rejects capability issuance. | Approval remains pending/non-final; no capability and no PO confirmation. | `VERIFIED V2` across the Odoo authority/SoD guard and typed issuer agent rejection; final-execution regression remains open. |
| `APP-N02` | Negative authority | Wrong-role, unauthenticated, cross-tenant or cross-company human attempts approval. | Trusted Odoo/PDP identity and current permission check reject issuance. | Approval remains non-final; no capability and no PO confirmation. | `VERIFIED V2` at the issuance guard for wrong role, PDP denial/outage and cross-tenant/company; final-execution regression remains open. |
| `APP-N03` | Negative workflow | Complete, delete or reassign the Activity without invoking authorized capability issuance. | Activity state is not accepted as AC v1. | Approval remains pending/non-final; no PO confirmation. | `DESIGNED V2` |
| `APP-N04` | Negative capability | Tamper/substitute payload or envelope; use expired, rejected, invalidated, superseded, unknown-key or consumed capability. | Typed AC verifier rejects before mutation. | No capability/command consumption and no final PO mutation. | `PARTIAL VERIFIED V2` for payload/envelope tamper, expiry and unknown key in focused tests plus persistent non-final Odoo tamper assertions; rejected/invalidated/superseded/consumed states remain V2-APP-04. |
| `APP-N05` | Negative key confusion | Present AC signature/approval key ID to delegation verifier or delegation proof to AC verifier; configure duplicate secrets. | Cross-protocol verification fails; detectable duplicate key material blocks startup. | No authorization state or PO mutation. | `PARTIAL VERIFIED V2` for delegation-key substitution at the AC verifier and duplicate approval/delegation/JWT/audit key-material rejection; reverse AC-to-delegation regression remains open. |

### 3.3 Commit-time transaction and boundary enforcement

| ID | Type | Setup/action | Required oracle | Expected persistent outcome | Status |
|---|---|---|---|---|---|
| `TXN-P01` | Positive final | Valid unchanged CBI, current authority/policy and unconsumed AC v1 reach final route. | Locked re-read matches; one transaction consumes command/capability and confirms PO. | PO reaches expected final state; approval is `consumed`; exactly one executed command/result is stored. | `DESIGNED V2` |
| `TXN-P02` | Positive retry | Response is lost after `TXN-P01` commits and identical command is retried. | Retry retrieves/reconstructs the terminal result without calling business mutation again. | Same single final PO effect and same terminal command/approval result. | `VERIFIED BASELINE` for bounded nonce retry; complete AC v1 path is `DESIGNED V2`. |
| `TXN-N01` | Negative concurrency | Two independent sessions execute the same approved command concurrently. | Row/conditional-state serialization allows at most one consumption/mutation. | One final PO effect, one consumed capability, one terminal command; loser returns safe terminal/conflict result. | `VERIFIED BASELINE` for nonce serialization; AC v1 atomicity is `DESIGNED V2`. |
| `TXN-N02` | Negative rollback | Inject business failure after conditional capability/command consumption but before transaction commit. | Entire database transaction rolls back. | PO remains non-final; capability and command remain safely retryable or consistently failed, never consumed alone. | `DESIGNED V2` |
| `TXN-N03` | Negative stale authority | Revoke/expire grant, deny current policy or remove approver permission after approval but before final execution. | Commit-time PDP verification rejects historical capability. | Approval may be marked invalidated separately; no final PO mutation or consumption as success. | `DESIGNED V2` |
| `TXN-N04` | Negative final race | Mutate amount/vendor/currency/line/state in another transaction between human approval and final route. | Final row lock/re-read produces different CBI/witness and rejects old capability. | New business edit may persist; old approval is unusable; no protected confirmation. | `DESIGNED V2` |
| `BOUND-N01` | Negative bypass | Invoke `button_confirm`, `button_approve` or supported API route directly without required V2 proof/AC. | Every in-scope final entry point reaches the PEP and fails closed. | PO remains non-final; no forged command/approval state. | `DESIGNED V2` |
| `BOUND-N02` | Negative availability | PDP/AC verifier is unavailable or reports degraded authorization during delegated final execution. | Final high-impact action fails closed; no stale cached allow is used. | No final PO mutation or successful consumption; controlled pending state may remain. | `VERIFIED V2` for initial PDP outage route; AC/final execution remains open. |
| `BOUND-N03` | Negative ambiguity | Approval row/capability is missing, duplicated, partially persisted or in an unknown state. | PEP rejects ambiguous authorization instead of guessing/recovering authority. | No final PO mutation; rows remain available for explicit recovery/invalidation. | `DESIGNED V2` |

The planned V2 set contains 25 bounded cases. `V2-EVAL-01` must turn this design into executable fixtures, may split tamper matrices into subtests and may add cases up to the agreed 20–30 scenario boundary without broadening the thesis workflow.

## 4. Existing evidence retained as baseline

| Property | Existing bounded evidence | Safe interpretation |
|---|---|---|
| Trusted principal/tenant | JWT negative tests and real Odoo-to-PDP mTLS boundary. | `VERIFIED BASELINE`; not enterprise IAM. |
| Current full-tuple proof | Go/Python v1 length-prefixed tuple tests and Odoo tampered-proof denial. | `VERIFIED BASELINE` for implemented fields; not CBI v1 binding. |
| Replay/idempotency | Unique `(tenant, grant, nonce)` attempt, altered-command replay denial and two-session serialization/retry. | `VERIFIED BASELINE` for one command path; not AC v1 atomic consumption. |
| Current SoD | Authenticated `env.user`, purchase-manager role, tenant/company, creator/delegator/agent separation and live-PDP approval decision are exercised over Odoo/mTLS. | `VERIFIED V2` through AC v1 issuance; final current-authority recheck remains open. |
| Revocation | Odoo live revoke and recorded local PostgreSQL multi-replica tests. | Bounded evidence only; not instant/universal revocation. |
| Approval routing | `to approve`, one attempt, one exact pending CBI row and one linked Activity assigned to an independently eligible manager persist without rollback or duplicate retry. | `VERIFIED V2` for APP-P01 and APP-P02 issuance: Activity alone is notification, while the authenticated issuance operation persists the AC v1 `approved` state and leaves the PO non-final. |
| PDP outage | Tested Odoo transaction fails closed when PDP is unavailable. | Bounded route behavior; not availability SLO. |
| Dynamic HR/daily limits | No authoritative input or executable path. | `NOT VERIFIED`; excluded from V2 claims. |

## 5. Comparative evaluation required by RQ4

All variants use the same Odoo fixture, purchase-order action, database/testbed revision and observation boundary.

| Variant | Authorization mechanism | Security expectation |
|---|---|---|
| **A — broad service account** | Authenticated service identity with broad final-action access; no transaction-bound delegated intent. | Establishes the over-privileged baseline and should fail substitution/replay/SoD protections by design. |
| **B — policy only** | Current identity/policy decision without CBI v1 + AC v1 transaction binding. | Can deny known attributes but should not satisfy stale-intent/exact-approval cases. |
| **C — proposed mechanism** | Current baseline plus CBI v1, AC v1 and commit-time atomic enforcement. | Must pass all applicable V2 cases without unauthorized persistent mutation. |

This comparison evaluates security correctness and added operational latency. It is not an OPA/Cedar benchmark and cannot establish general ERP superiority.

## 6. Current performance evidence retained for context

Historical fixed-latency and speedup claims are retired. The existing narrow warm purchase-confirmation measurement on commit `e243db5` is:

| Metric | Native Odoo + PostgreSQL | Odoo PEP + mTLS gRPC | Boundary |
|---|---:|---:|---|
| Mean, 750 confirmations | 29.933 ms | 71.525 ms | Warm checkpoint; final commit excluded. |
| p50 | 28.641 ms | 66.719 ms | Same recorded fixture. |
| p95 | 40.807 ms | 87.975 ms | No latency win claimed. |
| p99 | 48.843 ms | 101.640 ms | Not concurrent/production load. |

Raw samples and method limits: [`evidence/ODOO_ORM_COMPARISON_2026_09_15.md`](./evidence/ODOO_ORM_COMPARISON_2026_09_15.md).

V2-EVAL-03 must measure evaluator, proof/capability verification, gRPC/mTLS, locked state reconstruction and ERP mutation separately with p50/p95/p99/max, errors and environment metadata.

## 7. Claim limits

Passing this matrix may support bounded claims for the tested Odoo purchase-order path only. It does not establish production readiness, EU AI Act compliance, WORM retention, legal signature/non-repudiation, exactly-once external effects, prompt-injection prevention, SAP compatibility, multi-agent security or general ERP correctness.
