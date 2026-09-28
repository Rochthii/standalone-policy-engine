# Odoo Purchase-Order Final Transition Entry Points

> **Task:** V2-TXN-01
>
> **Platform boundary:** Repository-pinned Odoo 17 Community `purchase` module and the repository-owned `pdp_authorizer` addon.
>
> **Status:** `VERIFIED V2` for the repository-pinned Odoo 17 Community public ORM/RPC boundary. This does not prove final AC v1 execution or atomic consumption.

## In-scope transition

The bounded protected effect is the first transition of a delegated `purchase.order` from a non-final state (`draft`, `sent`, `to approve` or `cancel`) to `purchase` or `done`.

## Pinned Odoo call graph

| Public method/path | Pinned Odoo behavior | V2-TXN-01 treatment |
|---|---|---|
| `button_confirm` | Validates lines and calls `button_approve` when approval is allowed; otherwise writes `to approve`. | Supported public route; repository PEP evaluates before invoking the base implementation. |
| `button_approve` | Writes `purchase`, then may write `done` when PO locking is enabled. | Must fail at the state-write boundary unless invoked by the authorized internal base-confirm call. |
| `button_done` | Writes `done` directly. | Must fail when invoked on an in-scope non-final delegated PO. |
| `button_unlock` | Writes `purchase` directly. | Must fail when invoked on an in-scope non-final delegated PO. |
| Generic ORM/RPC `write` | Can supply `state` directly through the public model API. | Must fail for an in-scope non-final-to-final transition. |

The guard is placed at `purchase.order.write` so every listed base method converges on one enforcement point. The PEP passes a process-local object sentinel only while invoking Odoo's base `button_confirm`; an RPC caller can submit a context key but cannot reproduce object identity.

The `pdp_delegated_scope` flag is sticky. Creating or updating a PO with a delegation grant, agent or delegator marks the PO in scope, and later ORM/RPC writes cannot clear that scope. This prevents a two-step bypass that first removes delegation fields and then invokes a final-state method. Legacy rows with a current delegation marker are also protected even before the sticky field is backfilled.

## Verification

- Focused fresh-database public-dispatch suite: 6 post-tests / 8 test cases, 0 failures or errors.
- Full fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate: 26 post-tests / 34 test cases, 0 failures or errors.
- Negative cases assert persistent non-mutation for direct final methods, direct state writes, a serializable forged context value and delegation-marker removal.
- Positive regressions prove the supported low-value `button_confirm` route executes once and existing `purchase -> done -> purchase` operations remain available.
- mTLS boundary probes and the two-session check pass with one nonce, one executed attempt and one committed `purchase` state.

## Explicit limits

- Existing `purchase -> done` and `done -> purchase` operational transitions are not first-confirmation bypasses and remain outside this guard.
- Direct SQL, malicious trusted server code importing private module objects, and unreviewed third-party addons that replace the model contract are outside the external caller boundary.
- Odoo Enterprise and SAP paths are not evaluated.
- V2-TXN-01 only closes entry points. V2-TXN-02 through V2-TXN-04 provide separate bounded evidence for locked reconstruction, current authority, atomic consumption/mutation, rollback, two-session execution and lost-response retry; concurrent business-edit races remain open.
