# V2-EVAL-01 — Remaining boundary review

Date: 2026-09-26. Initial dispositions were source inspection; a subsequent fresh Odoo/mTLS run executed the controlled authority timing schedule below. See the [case ledger](V2_EVAL_01_CASE_LEDGER_2026_09_24.md).

## Latest disposition after the batched repair

Superseded on 2026-09-27: all retained EVAL-01 criteria now pass at their bounded, composed boundaries. TXN-N03 adds four after-ALLOW policy/role schedules and two real-clock deferred expiry rollback cases; the shared policy publication barrier and local-authority epoch close the retained ordering gap under the configured-writer assumptions. One fresh gate passes 75 post-tests, zero errors/failures and every runner (exit 0). The [ledger](V2_EVAL_01_CASE_LEDGER_2026_09_24.md) records exact scope, failed attempts and commands. The historical findings below remain history, not current open criteria.

## Historical disposition of the six partial rows

| Row / research question | Existing evidence and its actual boundary | Decision / minimum remaining work |
|---|---|---|
| AUTH-N01 / RQ1 | `grpc_server_test.go::TestGRPCServer_RequiredAuthentication` and `run-odoo-e2e.py::verify_mtls_boundary` reject missing JWT at CheckAccess. Final Odoo tests reject malformed JWT, wrong subject/tenant and cross-company grant reassignment with non-final/unconsumed state.   | Bounded verified as composed boundary evidence, not an ERP test for every RPC variant. Replace the HTTP middleware anchor with gRPC anchors. Agent JWT has no company claim: company is checked against the authoritative grant/order and signed CBI; adding a JWT claim is not a missing requirement. |
| AUTH-N02 / RQ1, RQ2 | Public final route covers revocation; proof tests cover malformed signature/validity. `test_approval_invalidation.test_expired_delegation_invalidates_without_business_effect` calls private revalidation. AC expiry is not grant expiry. | Partial. Exercise expired and inactive delegation after approval through the public final route, with PO/approval/attempt oracles. Keep proof TTL and grant lifetime evidence separately labelled. |
| CBI-N01 / RQ2 | Exact converter rejects float, exponent, precision and range errors before signing. Trusted reconstruction uses PostgreSQL NUMERIC text, not agent-supplied floating-point money. CBI-N05 rejects malformed wire money at live PDP; changed-price ERP test is not an exact one-minor-unit change. | Partial. Keep converter tests at serializer boundary; do not invent a public float-input path. Add a persisted approved-total change of exactly one minor unit, verified with actual currency scale and tax, then final denial without consumption. |
| APP-N02 / RQ3 | `test_approver_authority_and_sod_fail_closed` includes calls to `check_current_approver`; guard checks are not all public issuance calls. `TestApprovalCapabilityRPCFailsClosed` covers agent issuance, not missing-JWT issuance. Generic RequiredAuthentication tests target CheckAccess/ExplainDecision. | Partial. Add missing-JWT IssueApprovalCapability rejection/no capability and exercise retained wrong-role/cross-tenant/company negatives through public `action_issue_capability`, reusing fixtures. Assert no issued capability, no final PO and no executed command. |
| TXN-N03 / RQ1, RQ2, RQ3 | The after-ALLOW grant race was reproduced, then repaired with a shared ERP fence. Three independent-session schedules now verify revoke-first denial after retry, final-first blocking until commit, and rollback without consumption. | Partial: bounded grant-revocation ordering is verified. Live policy publication, role changes and expiry timing remain separate open criteria. |
| TXN-N04 / RQ2 | Four price/description schedules observe blocking, retry and fresh-session commit state. Existing-row locks are not proof of line membership or tax-relation protection. | Partial. Investigate insertion/deletion and tax-relation schedules with both relevant orderings. Map other retained material fields to their actual locking path before excluding redundant schedules. No full Cartesian product is required, but different lock paths cannot be dropped as duplicates. |

AUTH-N01 is closed only at the composed boundaries stated above; five rows remain partial. The 25 IDs remain unchanged. This is not a 25/25 security pass.

## Grant-revocation repair and remaining claim boundary

The initial diagnostic reproduced an unsafe commit after an independent-session revoke. The repaired route now locks a tenant/grant row in `pdp_delegation_fence_v1` until ERP commit. PDP revocation commits a monotonic tombstone in that Odoo database before its existing PDP publication. This gives final execution and grant revocation a shared database ordering point even though PDP policy storage is in another database.

The full gate passes 69 post-tests / 79 reported cases and three authority schedules: revoke after live ALLOW but before fence acquisition causes REPEATABLE READ retry and invalidation without consumption; final-first blocks the real revoke until business commit; final rollback releases the fence without consuming approval/command. Go/PostgreSQL evidence covers publication failure and cancellation while blocked. See the ledger for anchors and exact commands.

Every revoke writer for the protected grants must use the same configured ERP fence database. The PEP rejects missing/mismatched scope acknowledgement. If the ERP tombstone commits but PDP publication fails, final execution remains denied and revocation retry reconciles state. There is no distributed transaction across both databases, nor a guarantee that every timed-out revoke had no effect.

This repairs grant-revocation ordering. It does not establish atomic policy publication, role changes or expiry with ERP commit. Keep the wider normative invariant partial until those retained boundaries are resolved.

## Continue the same active task

EVAL-01 accepted on 2026-09-27; proceed to V2-DOC-05. Do not reopen completed rows or add nested task IDs. Broader recovery, arbitrary SQL/extensions, unconfigured writers and external effects remain outside the verified boundary.
