# ApprovalCapability v1 — Exact-Action Human Approval

> **Status:** V2 normative design with APP-P01/APP-P02 issuance evidence. Pending-record creation, the authenticated-human authority/SoD guard, purpose-separated AC v1 issuance/verification and the `pending -> approved` transition are verified for the initial Odoo/PostgreSQL/mTLS boundary. Invalidation, final revalidation and atomic consumption remain unimplemented.
> **Scope:** Human approval for one pending Odoo 17 purchase-order confirmation.
> **Intent contract:** [`CANONICAL_BUSINESS_INTENT.md`](./CANONICAL_BUSINESS_INTENT.md).
> **Evidence authority:** [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md).

## 1. Purpose and authority boundary

`ApprovalCapability v1` (AC v1) is cryptographic evidence that one authenticated, authorized human approved one already-bounded `CanonicalBusinessIntent` in one expected pending ERP state. It may satisfy an approval requirement but can never create, widen or restore delegated authority.

The final purchase-order transition remains valid only when all of these independently pass:

```text
current delegation authority
+ current policy and revocation state
+ reconstructed CanonicalBusinessIntent
+ current authorized human approver and SoD
+ valid unconsumed ApprovalCapability
+ atomic capability/command consumption with the ERP mutation
```

The Odoo route persists one exact post-transition pending CBI row and linked Activity as **VERIFIED V2 APP-P01** evidence. Neither that row nor the Activity is approval evidence by itself. A separate authenticated issuance operation now locks/reconstructs the same intent, reruns authority/SoD checks, receives and verifies AC v1 from the PDP, then persists `approved` without applying the protected purchase-order effect.

## 2. Issuer and trust model

1. The authenticated human acts through the trusted Odoo session. The PEP derives the approver from `env.user`; a form field, RPC parameter, prompt or agent-selected user is never authoritative.
2. Odoo locks and reconstructs the persisted post-transition pending CBI described in the intent contract. The record must already be in the expected `to approve` state; no lock is held while the human is deciding.
3. Odoo calls a PDP-side approval authority over the authenticated mTLS/JWT boundary as the human subject. The authority evaluates approval permission, tenant/company, current policy, active delegation and SoD before issuance.
4. The PDP-side authority owns the approval signing key. Odoo persists the returned capability and record but does not receive the approval signing secret.
5. HMAC authenticates capability bytes to configured key holders. It does not provide human legal signature, non-repudiation or protection after PDP signing-key compromise.

## 3. Canonical capability payload

| Ordinal | Field | Canonical type/rule | Authoritative source |
|---:|---|---|---|
| 1 | `capability_version` | Exact ASCII constant `ac.v1`. | Approval-authority implementation constant. |
| 2 | `purpose` | Exact ASCII constant `odoo.purchase_order.confirm`. | Approval route constant; never caller supplied. |
| 3 | `approval_id` | 128-bit or stronger CSPRNG value encoded as unpadded base64url. | Generated once by the trusted Odoo PEP when it creates the pending intent record; PDP binds the exact ID into the issued capability. |
| 4 | `tenant_id` | Exact non-empty tenant string; no trimming/case folding during verification. | Reconstructed CBI and authenticated JWT tenant; both must match. |
| 5 | `company_id` | Positive signed 64-bit integer. | Reconstructed pending CBI from locked `purchase.order.company_id.id`. |
| 6 | `intent_hash` | Raw 32-byte SHA-256 value represented externally as lowercase hex. | Hash of the exact persisted/reconstructed pending CBI bytes. |
| 7 | `state_witness` | Raw 32-byte SHA-256 value represented externally as lowercase hex. | Pending CBI state witness after transition to `to approve`. |
| 8 | `command_id` | Exact CBI command ID. | Persisted authorization-attempt idempotency key; legacy transport name `delegation_nonce`. |
| 9 | `delegation_grant_id` | Positive signed 64-bit integer. | Pending CBI and active `pdp.delegation.grant`; both must match. |
| 10 | `delegator_subject` | Exact `user:` subject from the active grant. | Reconstructed CBI and active grant. |
| 11 | `agent_subject` | Exact registered `agent:` subject. | Reconstructed CBI, active grant and original authenticated agent. |
| 12 | `creator_subject` | Exact `user:` subject. | Reconstructed CBI from `purchase.order.create_uid.login`. |
| 13 | `approver_user_id` | Positive signed 64-bit integer. | Authenticated Odoo `env.user.id`; never `sudo()`'s effective user or request input. |
| 14 | `approver_subject` | Exact `user:` subject. | `user:` plus authenticated `env.user.login`, cross-checked with PDP JWT `sub`. |
| 15 | `required_permission` | Exact ASCII constant `approval:purchase_order.confirm`. | Approval route/policy constant. |
| 16 | `issuance_policy_revision` | Non-negative signed 64-bit integer. | Tenant policy revision used by the PDP approval decision. It is audit context, not permission to skip the final current-policy check. |
| 17 | `issued_at` | UTC Unix seconds, positive signed 64-bit integer. | PDP-side trusted clock at successful issuance. |
| 18 | `expires_at` | UTC Unix seconds greater than `issued_at`. | PDP computes `min(issued_at + configured approval TTL, delegation valid_until)`; configured TTL must be positive and no more than 24 hours. |
| 19 | `one_time_id` | Independent 256-bit CSPRNG value encoded as unpadded base64url. | Generated by the PDP-side approval authority; persisted with a uniqueness constraint. |

The payload intentionally binds both `intent_hash` and `state_witness`. The former protects the complete command; the latter makes the expected pending state explicit and easy to compare at execution. Repeating selected identity fields supports direct SoD and tenant checks but never replaces comparison with the complete CBI.

## 4. Canonical encoding and envelope

The canonical payload starts with the byte domain separator `PDP-APPROVAL-CAPABILITY-V1`. Fields follow the exact ordinal order in Section 3 using the CBI canonical primitives: length-prefixed UTF-8 strings, signed 64-bit big-endian integers and raw 32-byte digests.

The transport envelope contains:

| Field | Rule |
|---|---|
| `algorithm` | Exact `HS256`; no algorithm supplied by a caller is dynamically selected. |
| `key_id` | Approval-ring key ID matching `^[A-Za-z0-9_-]{1,64}$`. |
| `payload` | Exact canonical AC v1 bytes or an unambiguous base64url representation. |
| `signature` | `HMAC-SHA256(approval_key[key_id], canonical_payload)` compared in constant time. |

An implementation may choose a compact transport representation, but Go and Python must decode to identical canonical bytes. Unknown versions, algorithms, key IDs, duplicated fields, malformed lengths and non-canonical encodings fail closed.

## 5. Purpose-separated keys and verifier domains

Approval capability keys and delegation proof keys are separate security domains:

- the approval authority loads a dedicated approval key ring and active approval key ID;
- approval key material must not equal any delegation-proof key, JWT key or audit-encryption key; startup fails on a detectable duplicate;
- approval key IDs use their own namespace and cannot resolve through the delegation key ring;
- the typed AC verifier accepts only the approval domain separator and `ac.v1`; the delegation verifier accepts neither;
- the approval signer API accepts only a validated approval payload after approver authorization; it cannot sign arbitrary bytes or delegation tuples;
- rotation issues with one active key and temporarily retains older keys as verify-only for no longer than the maximum capability TTL plus bounded clock skew;
- removal of a still-needed verification key invalidates affected capabilities and must fail closed visibly.

Domain separation prevents signature confusion even if implementation defects accidentally route bytes to the wrong verifier. Distinct secret material limits cross-protocol impact. Because the current architecture uses one PDP process, host/process compromise can still expose both rings and remains outside the thesis guarantee.

## 6. Separation of Duties and approver authorization

Issuance and final execution both enforce all rules below against authoritative subjects:

1. `approver_subject` must be a human `user:` subject in the same tenant and company as the pending CBI.
2. The approver must currently possess `approval:purchase_order.confirm` through trusted Odoo group mapping and the PDP policy decision. A displayed role name or caller-provided permission is insufficient.
3. The approver must differ from `agent_subject`, `creator_subject` and `delegator_subject`. Comparison uses canonical subjects and linked immutable Odoo user IDs where available.
4. The AI agent cannot select, impersonate or delegate to the approver, and cannot invoke the human approval endpoint with an agent JWT.
5. Odoo administrator/superuser status does not silently bypass SoD. Any future break-glass path requires a separate purpose, policy, audit and evaluation and is outside AC v1.
6. Approval does not override grant scope, expiry, revocation, tenant isolation, proof validation or current policy. If any underlying authority fails, the capability is unusable.

**Implemented boundary (2026-09-21):** Odoo derives the human from authenticated `env.user`, locks the PO and approval rows, reconstructs the unchanged pending CBI, and reruns same-tenant/company, active internal purchase-manager, creator/delegator/agent separation and live-PDP checks for `action:APPROVE_PURCHASE_ORDER`. The PDP then issues and verifies a typed AC v1 under its dedicated approval key. Odoo stores the exact returned binding and moves only the approval row to `approved`; the linked Activity remains notification and the PO remains `to approve`.

If a deployment intentionally permits a delegator or creator to approve, that is a different SoD policy and cannot reuse the AC v1 thesis claim without changing the policy, threat model and negative-test matrix.

## 7. Persistent approval record and state machine

Odoo creates one approval record with `approval_id` and the exact pending CBI before a human acts. On issuance it adds the remaining payload fields, exact capability bytes/signature hash, current state, timestamps and terminal reason. Database constraints require unique `approval_id` and unique `(tenant_id, one_time_id)`; one pending command may have at most one active approved capability.

```text
pending  -> approved -> consumed
pending  -> rejected
pending  -> invalidated
approved -> invalidated
approved -> expired
approved -> consumed
```

- `pending` means an exact post-transition CBI is waiting for a human; it is not authorization.
- `approved` means AC v1 was issued after the human and SoD checks passed.
- `consumed` is terminal and is written atomically with the protected purchase-order mutation.
- `rejected`, `invalidated` and `expired` are terminal and never produce a final business effect.
- Reissuing after a terminal state requires a new `approval_id`, `one_time_id`, current CBI reconstruction and complete authorization. The old row remains immutable evidence.

State transitions use a locked row or conditional update. A check-then-write sequence without database serialization is invalid because two sessions could consume one approval.

**Implemented transition:** `pending -> approved` is verified for one unchanged intent, and an identical retry returns the already-verified capability without issuing a second one. `rejected`, `invalidated`, `expired` and `consumed` transitions remain APP-04 or transaction-phase work.

## 8. Issuance algorithm

1. Authenticate the Odoo human session and derive `approver_user_id`/`approver_subject` from `env.user`.
2. Lock the purchase order and pending approval row; invalidate ORM caches.
3. Reconstruct the post-transition CBI and require exact equality with the persisted pending intent, including `state_witness` and `command_id`.
4. Validate the active one-hop grant, proof metadata, tenant/company and current revocation state.
5. Apply every SoD and approver-permission rule in Section 6 using the authenticated human subject.
6. Ask the PDP-side approval authority to evaluate current policy and issue AC v1 with its own active approval key.
7. Persist the canonical payload, envelope/fingerprint and `approved` state in the same transaction. Commit no protected purchase-order effect.

An Activity may notify or navigate the human to this operation, but completing an Activity alone never changes the approval record to `approved`.

**Implementation evidence:** The typed issue RPC authenticates tenant/subject/company, checks live revocation and grant expiry, evaluates current approval policy and signs only the fixed AC purpose/permission. The typed verifier rejects payload/envelope tamper, expiry, unknown key, wrong algorithm/version/purpose and delegation-key substitution. Odoo receives no approval signing key.

## 9. Final verification and atomic consumption

> **Implementation status:** Not implemented. AC v1 issuance-time verification does not authorize a final purchase-order mutation.

1. Authenticate the final route and lock the purchase order, command row and approval row in a documented consistent order.
2. Require the approval record state to be `approved`, then reconstruct current CBI from locked ERP data and compare `intent_hash`, `state_witness`, `command_id`, grant and subjects byte-for-byte.
3. Submit the opaque capability and reconstructed context to the typed PDP-side AC verifier; Odoo does not verify HMAC locally or receive approval key material.
4. The PDP verifier requires a known approval key, valid signature, unexpired time and matching tenant/company/approval identifiers, then re-evaluates current delegation, approver permission/SoD, policy and revocation. `issuance_policy_revision` is not a stale-policy exemption.
5. Conditionally transition the approval from `approved` to `consumed` and consume the command ID in the same Odoo/PostgreSQL transaction as `button_confirm`/final mutation.
6. If any validation or business mutation fails, roll back capability/command consumption and the mutation together.
7. A lost-response retry reads the committed terminal result. It never replays the mutation or resets a consumed capability.

## 10. Invalidation and failure rules

The final action fails closed when any of the following is true:

- capability bytes/signature, purpose, version, key ID or any payload field are malformed or changed;
- intent hash, state witness, command ID, tenant, company, grant or subjects differ;
- the PO/vendor/currency/amount/lines/state/write version changed after approval;
- the grant expired, was revoked, became inactive or no longer covers the action;
- the approver lost required permission, moved tenant/company or now violates SoD;
- current PDP policy denies or cannot be reached under the fail-closed rule;
- capability expired, was rejected, invalidated, superseded or already consumed;
- approval verification key is unknown/retired or approval state is ambiguous;
- a direct UI, API or workflow route attempts final mutation without AC v1 when approval is required.

Invalidation may be detected eagerly by an event or lazily during final verification. The security requirement is denial before mutation, not instant cache propagation.

## 11. Required evidence

| Case | Required result |
|---|---|
| Authorized independent human approves unchanged pending CBI. | One AC v1 record becomes `approved`; no final mutation yet. |
| Agent, creator, delegator, wrong role or cross-tenant user attempts approval. | No capability issued; state remains non-final. |
| Change intent or material ERP state after approval. | Old capability invalid; no mutation. |
| Expire/revoke grant or remove approver permission. | Final execution denied despite valid historical signature. |
| Tamper any payload/envelope field or substitute capability. | Signature/context verification fails. |
| Present approval key material/signature to delegation verifier, or vice versa. | Cross-protocol verification fails. |
| Reuse or concurrently consume one capability. | At most one committed purchase-order effect. |
| Business mutation rolls back after conditional consumption. | Consumption also rolls back; safe retry remains possible. |
| Response is lost after commit. | Retry returns/reconstructs terminal result without a second effect. |
| Complete/delete/reassign an Odoo Activity without capability issuance. | No approval and no final effect. |

## 12. Explicit limits

AC v1 does not establish legal electronic signature, non-repudiation, WebAuthn, dual-control/multiple approvers, cross-organization approval, SAP workflow compatibility, exactly-once external effects or production readiness. It covers one human approval for one Odoo purchase-order confirmation within the tested transaction boundary.
