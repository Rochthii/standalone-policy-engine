# Delegation Replay and Idempotency Contract

> **Decision date:** 2026-09-12  
> **Status:** Verified in the PDP and repository-owned Odoo PEP for duplicate retry, altered-command replay, rollback and two-session serialization-retry behavior against PostgreSQL.

## 1. Decision

`CheckAccess` is an idempotent policy-decision operation. It validates the signed delegation tuple but does not consume `delegation_nonce`. Rejecting a nonce inside the PDP would create an unsafe failure mode: when an ALLOW response is lost, a legitimate retry would be rejected even though the ERP action never executed.

Replay safety is enforced at the Policy Enforcement Point, where the authorization check and the protected ERP mutation share one database transaction. A caller that bypasses the PEP can obtain only a decision; it cannot execute the protected Odoo operation.

## 2. Nonce contract

- `delegation_nonce` identifies one intended ERP business command, not one network attempt.
- The nonce is required for delegated calls and is covered by the HMAC proof.
- Retries of the same business command reuse the same nonce and exact signed tuple.
- A different business command must use a new cryptographically random nonce.
- The PEP uniqueness scope is `(tenant_id, delegation_grant_id, delegation_nonce)`.
- A nonce record is retained at least until `delegation_valid_until`, plus the operational retry grace period.

## 3. Required Odoo transaction flow

1. Begin the normal Odoo/PostgreSQL transaction.
2. Insert an authorization-attempt row with a unique constraint on the nonce scope.
3. If the row already exists, return the recorded business outcome without re-executing the protected mutation.
4. Call `CheckAccess` using the tuple stored in that attempt row.
5. On ALLOW, execute the ERP mutation and mark the attempt `executed` in the same transaction.
6. On an approval obligation, persist `to approve`, create the Activity and mark the attempt `approval_required` without raising an exception.
7. On hard DENY or PDP failure, fail closed. Transaction rollback removes an uncommitted attempt, so a request whose decision response was lost can retry safely.

Odoo executes RPC transactions at an isolation level where the losing concurrent
session can receive PostgreSQL `SerializationFailure`. The standard
`odoo.service.model.retrying` wrapper must retry the whole transaction; retrying
inside the already-aborted cursor is invalid. The real two-session gate verifies
that this produces one nonce, one completed attempt and one protected mutation.

This transaction pattern makes execution exactly-once with respect to the Odoo database even though gRPC delivery and `CheckAccess` are at-least-once/idempotent.

## 4. Key-ring and proof envelope

Proofs use the envelope `v1.<kid>.<hex-hmac>`. The `kid` is validated, selected from the configured key ring and included in the canonical HMAC bytes.

Configuration:

| Key | Meaning |
|---|---|
| `PDP_DELEGATION_ACTIVE_KID` | Key ID used to sign new proofs |
| `PDP_DELEGATION_KEYS_JSON` | JSON object mapping accepted key IDs to secrets |
| `PDP_SHARED_SECRET` | Development/test compatibility fallback only; forbidden as the production rotation mechanism |

Rotation sequence:

1. Add the new key while retaining the old key; deploy verifiers.
2. Change `PDP_DELEGATION_ACTIVE_KID`; deploy signers.
3. Keep the old verification key for at least the maximum proof TTL and clock-skew/retry grace period.
4. Remove the old key and verify that proofs carrying its `kid` fail closed.

## 5. Security boundaries

- Key-ring rotation prevents verification outages and ambiguous secret replacement; it does not itself provide execution replay safety.
- The current PDP revocation map remains process-local and is a separate durability gate.
- Duplicate, altered-command, rollback and concurrent serialization-retry cases pass in real Odoo/PostgreSQL transactions; this does not imply durable multi-replica PDP revocation.
- Logs must never contain proof values, nonces or key material.
