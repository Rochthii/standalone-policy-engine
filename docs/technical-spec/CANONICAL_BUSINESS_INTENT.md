# CanonicalBusinessIntent v1 — Odoo Purchase-Order Confirmation

> **EVAL-02 snapshot stabilization (2026-09-27):** The builder now flushes all pending line fields before reading CBI, including deferred related-state recomputation that can update line write versions. This fixes false invalidation across separately committed pending, issuance and execution phases without excluding write versions from the witness. The three-transaction regression and post-review negatives pass; see [EVAL-02 evidence](evidence/V2_EVAL_02_COMPARISON_2026_09_27.md). The latest current-state audit supersedes historical open-race statements below.

> **Status:** Implemented and verified for the initial delegated Odoo PO-confirm route, post-approval intent persistence, AC v1 issuance and a bounded approved final `button_confirm` route that calls locked intent preflight and current-authority checks before same-transaction consumption/mutation. Two-session execution, injected rollback and lost-response retry pass; concurrent business-field edit races remain open.
> **Scope:** One delegated Odoo 17 `purchase.order` confirmation inside one Odoo/PostgreSQL transaction.
> **Evidence authority:** [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md).

## 1. Purpose and decision boundary

`CanonicalBusinessIntent v1` (CBI v1) is the exact business command that delegation proof, policy evaluation and any human approval must protect. The trusted Odoo PEP constructs it from authoritative records; the AI agent may request an operation but cannot supply protected values.

CBI v1 prevents an observed authorization for one purchase order from being reused after a material change to its action, tenant/company, vendor, currency, amount, lines or persisted state. It does not establish authorization until the proof, current policy/revocation state and required approval also pass.

The bounded delegated Odoo route now uses CBI v1 and requires proof V2 before policy evaluation, including the approved final route's fresh agent decision. V1 remains only for an explicitly tested legacy boundary whose resource is not a canonical numeric Odoo purchase-order identifier. The current evidence covers approved two-session execution, rollback and retry for one PO; independent-session business-field edit races remain open.

## 2. Trust and source rules

1. The PEP identifies the record through Odoo routing and access control, then obtains protected values from the persisted ORM/PostgreSQL record. Prompt text, tool arguments and arbitrary request context are never authoritative.
2. Immediately before final execution, the PEP locks the target `purchase_order` row, invalidates relevant ORM cache entries and reconstructs CBI v1 in the same database transaction that may commit the mutation.
3. Stored PostgreSQL `NUMERIC` values are converted through exact decimal arithmetic. A binary floating-point value is never serialized, signed or compared as an authorization amount.
4. Missing, unsupported, ambiguous or non-canonical values fail closed. An installed module that adds a field capable of changing confirmation semantics requires an explicit serializer update and schema-version review.
5. CBI contains current business state. Delegation validity, proof key ID, proof signature, approval signature and current policy revision belong to their envelopes/checks, not to agent-controlled context.

## 3. Top-level schema

| Ordinal | Field | Canonical type/rule | Authoritative Odoo/PostgreSQL source |
|---:|---|---|---|
| 1 | `intent_version` | Exact ASCII constant `cbi.v1`. | PEP implementation constant; never accepted from the caller. |
| 2 | `tenant_id` | Non-empty UTF-8 after rejecting leading/trailing whitespace; no case folding. | `purchase.order.company_id.pdp_tenant_id`. It must equal the authenticated JWT tenant and grant tenant. |
| 3 | `company_id` | Positive signed 64-bit integer encoded as canonical base-10. | Locked `purchase.order.company_id.id`. |
| 4 | `resource_type` | Exact ASCII constant `purchase.order`. | PEP route/model constant; `order._name` must match. |
| 5 | `resource_id` | Positive signed 64-bit integer encoded as canonical base-10. | Locked `purchase.order.id`. The PDP resource is derived as `purchase_order:<resource_id>`. |
| 6 | `action` | Exact ASCII constant `action:CONFIRM_PURCHASE_ORDER`. | PEP method/route constant protecting `button_confirm` and the supported final approval route. The existing `action:APPROVE_PURCHASE_ORDER` is a v1 migration concern, not the CBI v1 value. |
| 7 | `vendor_id` | Positive signed 64-bit integer encoded as canonical base-10. | Locked `purchase.order.partner_id.id`. Display name, prompt text and caller-provided vendor data are excluded. |
| 8 | `currency_code` | Uppercase three-letter ASCII ISO 4217 code. Reject missing/non-conforming codes. | `purchase.order.currency_id.name` from the referenced `res.currency`. |
| 9 | `currency_scale` | Integer from 0 through 6. | Trusted `purchase.order.currency_id.decimal_places`, derived from configured currency precision. |
| 10 | `amount_minor` | Signed 64-bit integer; for this workflow it must be non-negative. No float, decimal point or exponent is allowed. | Locked stored `purchase_order.amount_total` read as PostgreSQL `NUMERIC`, converted by Section 4 using the trusted currency scale. |
| 11 | `line_digest` | Lowercase 64-character hex SHA-256 digest of the line snapshot in Section 5. | Locked/re-read persisted `purchase.order.order_line` records. |
| 12 | `record_state` | Exact stored selection key; for initial confirmation the expected value is an allowed pre-confirmation state defined by the PEP. | Locked `purchase.order.state`; never a UI label. |
| 13 | `record_write_version` | UTC timestamp `YYYY-MM-DDTHH:MM:SS.ffffffZ`; missing value fails closed. | Locked PostgreSQL `purchase_order.write_date`. It is a concurrency signal, not the sole state witness. |
| 14 | `state_witness` | Lowercase 64-character hex SHA-256 digest defined in Section 6. | PEP recomputation from the locked material snapshot. |
| 15 | `creator_subject` | `user:` plus the exact persisted login; empty login fails closed. | `purchase.order.create_uid.login`. Used for SoD evaluation. |
| 16 | `delegation_grant_id` | Positive signed 64-bit integer encoded as canonical base-10. | `purchase.order.delegation_grant_id.id`, cross-checked against the active `pdp.delegation.grant`. |
| 17 | `delegator_subject` | `user:` plus the exact persisted login. | Active grant `user_id.login`; grant company/tenant must match the order. |
| 18 | `agent_subject` | Non-empty registered subject with the required `agent:` prefix. | Active grant `agent_id`, cross-checked against `purchase.order.ai_agent_id` and authenticated PDP subject. |
| 19 | `command_id` | 256-bit CSPRNG value encoded as unpadded base64url; immutable for retries of the same logical command. | Generated by the trusted PEP and persisted as the authorization-attempt idempotency key. The current transport field `delegation_nonce` is its legacy name. |
| 20 | `proof_version` | Exact ASCII constant selected by the V2 proof implementation, initially `v2`. | PEP/signer configuration; must match the verifier and cannot silently downgrade. |

`partner_id`, currency, company and relation IDs are stable record identities inside the tested Odoo database. Changes to vendor master data outside the purchase-order fields, such as a bank-account edit, are not covered by CBI v1 and must not be claimed as protected.

## 4. Money normalization

Let `A` be `amount_total` obtained as exact PostgreSQL `NUMERIC`, and let `S` be `currency_scale`.

```text
scaled = A * 10^S
require scaled is an integer
require 0 <= scaled <= MaxInt64
amount_minor = int64(scaled)
```

Before conversion, the amount must satisfy the configured Odoo currency rounding rule. Rounding an unauthorized input inside the serializer is forbidden: a non-integral `scaled` value fails closed instead of being rounded silently.

Examples:

| Stored amount | Currency/scale | `amount_minor` |
|---:|---|---:|
| `1234.56` | USD / 2 | `123456` |
| `5000` | JPY / 0 | `5000` |
| `12.345` | KWD / 3 | `12345` |

The historical `ceil(float(amount_total))` behavior is explicitly incompatible with CBI v1 because it loses fractional currency and can authorize a different value from the persisted transaction.

## 5. Line snapshot and digest

All persisted `purchase.order.line` rows belonging to the order are sorted by `(sequence, id)` and encoded. Each line contains the following fields in this fixed order:

| Field | Canonical rule/source |
|---|---|
| `line_id` | Positive base-10 `purchase.order.line.id`. |
| `sequence` | Signed base-10 `sequence`. |
| `display_type` | Exact stored key or empty string. |
| `product_id` | Positive base-10 ID, or `0` only for a permitted section/note line. |
| `description` | Exact Unicode `name` normalized to NFC; line endings normalized to LF. |
| `uom_id` | Positive base-10 `product_uom.id`, or `0` only where the model permits no UoM. |
| `quantity` | Exact PostgreSQL `NUMERIC` `product_qty` in canonical decimal form: no exponent, no leading plus, no insignificant trailing zero, and zero encoded as `0`. |
| `unit_price` | Exact PostgreSQL `NUMERIC` `price_unit` using the same canonical-decimal rule. It is not forced to currency minor units because Odoo may retain finer unit-price precision. |
| `tax_ids` | Numeric `taxes_id` values sorted ascending and encoded as a length-delimited list. |
| `planned_at` | `date_planned` normalized to UTC microseconds as above, or empty only if the Odoo model explicitly permits it. |
| `line_write_version` | PostgreSQL `purchase_order_line.write_date` normalized to UTC microseconds. |

The line encoding starts with domain separator `PDP-ODOO-PO-LINES-V1`, then the line count, then every field using the canonical primitives in Section 7. `line_digest = SHA-256(line_encoding)`.

Any installed extension that changes price, taxes, quantity, vendor obligation or confirmation behavior through an additional line field must either add that field in a new intent version or make the protected route fail closed. The v1 serializer must not ignore an unknown material extension merely because `amount_total` happens to remain unchanged.

## 6. Material-state witness

The state snapshot starts with domain separator `PDP-ODOO-PO-STATE-V1` and includes, in fixed order:

```text
tenant_id
company_id
resource_type
resource_id
vendor_id
currency_code
currency_scale
amount_minor
line_digest
record_state
record_write_version
delegation_grant_id
creator_subject
```

`state_witness = SHA-256(canonical_state_snapshot)` and is included in CBI v1. This combines semantic values with persisted write timestamps: a material change is detected by value, and a changed timestamp also invalidates the old intent. Odoo `write_date` is not a guaranteed monotonic counter for every unrelated ORM write; an unchanged timestamp alone does not prove no intervening write. Fields outside the declared CBI material set are not protected merely because they may sometimes update `write_date`.

At final execution the PEP must lock and re-read the record, rebuild the line digest and witness, and compare the reconstructed complete intent to the pending/approved intent. `write_date` alone is insufficient because line updates may not reliably advance the parent timestamp.

## 7. Canonical byte encoding and hashes

The encoding is language-neutral and follows these primitives:

- byte prefix `PDP-CANONICAL-BUSINESS-INTENT`;
- fields appear exactly in the ordinal order in Section 3;
- UTF-8 strings are preceded by unsigned 32-bit big-endian byte length;
- signed integers use 64-bit big-endian two's-complement encoding;
- lists start with unsigned 32-bit big-endian item count;
- digests are encoded as their 32 raw bytes after strict lowercase-hex decoding;
- timestamps and canonical decimals are length-prefixed ASCII strings;
- no locale formatting, JSON map ordering, implicit trimming, case folding or Unicode normalization is allowed except where explicitly stated.

```text
intent_hash = SHA-256(canonical_intent_bytes)
```

The V2 delegation proof must bind `intent_hash`, grant/validity/key metadata and the authenticated agent. `ApprovalCapability v1` will bind this same `intent_hash` and `state_witness`; it may not reproduce a weaker subset of the intent.

## 8. Reconstruction and comparison algorithm

CBI has two valid workflow uses:

- **Direct allow:** construct and authorize from the locked pre-confirmation state, then apply the confirmation in the same transaction.
- **Human approval:** the initial decision may create only the non-final pending state. In that same transaction, after writing `to approve`, the PEP re-reads the resulting record, constructs the pending CBI/witness and persists it for review. The human approves that post-transition intent. Final execution requires the record to remain in that expected pending state and reproduce the same canonical intent before applying the protected confirmation.

An intent constructed from the earlier draft state must never be treated as approval for a later `to approve` state. Creating an Activity cannot substitute for persisting the exact post-transition intent.

1. Begin or join the Odoo transaction and identify one persisted purchase order.
2. Acquire a row lock on `purchase_order`; do not hold this lock while waiting for a human.
3. Invalidate cached protected fields and re-read the order, grant and lines from the trusted database boundary.
4. Validate tenant/company, record/action route, active one-hop grant and subject relationships.
5. Convert stored monetary/decimal values using Sections 4–5; reject unsupported precision or range.
6. Build `line_digest`, `state_witness`, canonical intent bytes and `intent_hash`.
7. Compare all canonical bytes or the collision-resistant hash with the proof/pending approval payload.
8. Recheck current policy and revocation. Do not treat a prior policy revision as current authority.
9. If approval is required, verify an exact-action approval capability for the same intent hash and witness.
10. In one transaction, atomically consume the command/approval and apply the final mutation; otherwise roll back and fail closed.

## 9. Required negative and interoperability evidence

| Case | Required result |
|---|---|
| Go and Python serialize the same fixture. | Identical canonical bytes and hashes. |
| Change action, resource, tenant or company. | Intent hash changes; old proof fails. |
| Change amount by one minor unit. | Intent hash changes; old proof/approval fails. |
| Change currency with numerically similar amount. | Intent hash changes. |
| Change vendor. | Intent hash changes. |
| Add/remove/reorder or materially edit a line. | Line digest and intent hash change. |
| Change PO state or either write version. | State witness and intent hash change. |
| Replay command ID for different canonical bytes. | Fail closed without business mutation. |
| Retry the same command ID and canonical bytes after a committed result. | Return/recover the recorded result without a second mutation. |
| Supply a float, exponent, overflow, malformed currency or unknown material extension. | Fail closed before authorization. |
| Attempt V1 downgrade on the protected route. | Fail closed. |

## 10. Explicit exclusions

CBI v1 does not bind vendor bank accounts, external payment instructions, attachments, chatter, arbitrary prompt text, SAP objects or external side effects. It does not prove non-repudiation, production readiness or general ERP correctness. Adding any excluded field to the protected business effect requires a schema/version change and new cross-language/tamper evidence.
