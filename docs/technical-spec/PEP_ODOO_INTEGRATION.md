# Odoo 17 PEP Integration

> **Updated:** 2026-09-27
> **Status:** EVAL-01 bounded gate passes: 75 post-tests, zero failures/errors, existing retry/rollback/outage checks, 16 enumerated material-edit schedules and grant/policy/role/deferred-expiry ordering. [Exact evidence](./evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md). All authority writers must share the configured ERP fence; see the addon README for fail-closed publication recovery and precise deadline semantics. This is not general distributed atomicity.

The authoritative addon is
[`custom_addons/pdp_authorizer`](../../custom_addons/pdp_authorizer). The external
Project 2 addon is historical input only and is incompatible with the current
security and Protobuf contract.

## 1. Enforcement boundary

`purchase.order.button_confirm()` is the protected business boundary. The PEP:

1. resolves the exact PDP tenant configured on `res.company`;
2. derives the subject, resource and trusted Odoo resource attributes;
3. creates a short-lived tenant-bound JWT for the effective user or AI agent;
4. for protected delegated execution, reconstructs CBI and signs its V2 proof with the active delegation key;
5. inserts the nonce ledger row before `CheckAccess`;
6. executes directly only on `ALLOW` without obligations, or on a later approved call after locked CBI/AC/current-authority rechecks and a fresh agent `ALLOW`;
7. commits `to approve` without raising when the delegated `ALLOW` carries `REQUIRE_HUMAN_APPROVAL`; and
8. raises `AccessError` on hard deny or PDP failure so the transaction rolls back.

The PDP remains a decision service. Scoped ERP at-most-once behavior depends on
the Odoo transaction and ledger; approved-final race/retry evidence is bounded to one PO and does not cover external effects. See
[`DELEGATION_REPLAY_IDEMPOTENCY.md`](./DELEGATION_REPLAY_IDEMPOTENCY.md).

## 2. Replay-safe transaction state

`pdp.authorization.attempt` has a database unique constraint on:

```text
(tenant_id, delegation_grant_id, delegation_nonce)
```

Nonce creation first locks the target `purchase_order` row with `FOR UPDATE`.
Concurrent confirmations therefore reuse the same business-command nonce rather
than racing to create different valid nonces.

The row also stores a SHA-256 fingerprint of the canonical business tuple,
business model/record identity, validity deadline and one terminal state:

- `executed`: the protected ORM mutation completed in the same transaction;
- `approval_required`: the non-rollback approval state and Activity were written; `decision` records the actual PDP `ALLOW` on the protected delegated high-value route, not a fabricated denial.

A retry with the same tuple returns the recorded outcome without re-executing
the mutation or scheduling a duplicate Activity. Reusing the nonce with a
different tuple or business record fails closed. An uncommitted `pending` row is
rolled back with a hard deny, PDP outage or ORM failure.

## 3. Identity, proof and transport

- RPC metadata is `Authorization: Bearer <JWT>`.
- JWT `sub` equals the effective Odoo user or delegated AI agent; the server
  replaces the request `subject` with this signed value.
- Human `principal.department` comes from the Odoo user `PDP Department` field
  and is embedded in the signed JWT; it is never trusted from request context.
- JWT tenant, issuer and audience must match the PDP configuration.
- Delegated calls include every field required by
  [`PROTOCOL_CONTRACT.md`](./PROTOCOL_CONTRACT.md), including nonce and validity window.
- The protected numeric Odoo PO route requires CBI-bound proof `v2.<kid>.<hmac>`;
  V1 is retained only for the tested legacy non-numeric resource boundary.
- The client recreates its channel after a PID change and uses a 350 ms deadline.
- Development/test may use insecure transport. Production startup rejects a
  client configuration without CA, certificate and private key.

Generated files are not copied into the addon. `clients/python` is the single
generated Python contract and is mounted read-only at `/opt/pdp-clients` with
that path in `PYTHONPATH`.

## 4. Decision behavior

| PDP result | Odoo behavior | Transaction outcome |
|---|---|---|
| `ALLOW` without obligations | call the native confirmation and mark attempt `executed` | commit together |
| `ALLOW` + `REQUIRE_HUMAN_APPROVAL` | write `state=to approve`, persist exact pending intent and Activity, mark `approval_required` | commit non-final; no exception |
| Approved retry + fresh agent `ALLOW` | lock/recheck CBI and AC/current authority; consume AC and command with the guarded PO final transition | one Odoo transaction; bounded two-session race/rollback/retry evidence passes |
| Any delegated `DENY`, even with approval obligation | raise `AccessError`; AC cannot override a hard forbid | rollback |
| Legacy non-delegated `DENY` + approval obligation | retain non-final `to approve` routing only | commit non-final; no protected agent final effect |
| RPC/auth/config failure | fail closed | rollback |
| repeated completed nonce | return recorded outcome | no duplicate mutation |

Obligations are read as typed Protobuf messages (`type`, `message`, `payload`),
not compared as raw strings.

## 5. Configuration and testbed

The complete variable list and operator notes are in the
[addon README](../../custom_addons/pdp_authorizer/README.md). The repository
Compose file now mounts both the addon and the canonical Python generated
client from repository-local paths. Its Odoo image installs the pinned Python
runtime dependencies.

## 6. Required evidence before verification

The code must not be marked production-ready until a fresh Odoo database proves:

1. addon install/upgrade succeeds;
2. ALLOW performs one native confirmation;
3. hard deny rolls back the protected mutation;
4. approval obligation persists `to approve` and one Activity;
5. retrying a completed nonce does not repeat either outcome;
6. a different tuple with the same nonce fails closed;
7. PDP outage fails closed without consuming the command nonce;
8. tampered and revoked proofs fail; and
9. the same suite runs with mTLS enabled.

The repository contains seven real-boundary transaction tests covering items
1–8 plus a two-session runner for the concurrent form of item 5. The Compose
gate recreates an isolated `odoo_e2e` database and seeds the live PDP. Run it
with:

```bash
make test-odoo-e2e
```

On 2026-09-12 the suite generated a short-lived test CA and client/server
certificates, rejected a client without a certificate, installed the addon in a
freshly recreated Odoo 17 database, passed all seven transaction cases with 0
failures/errors and passed the two-session serialization-retry assertion with
one nonce, attempt and mutation. The exact command, environment and result are
recorded in
[`evidence/ODOO_E2E_2026_09_12.md`](./evidence/ODOO_E2E_2026_09_12.md).
