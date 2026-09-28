# Odoo 17 PDP Authorizer

This addon is the repository-owned Policy Enforcement Point (PEP) for purchase
order confirmation. It replaces the incompatible Project 2 prototype.

## Thesis role

**Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17**

*Transaction-Bound Authorization for AI-Agent Actions in ERP: Design and Evaluation on Odoo 17*

The AI agent is a non-human software principal acting under controlled human delegation. This addon enforces the three contribution layers: delegation, authoritative intent binding and commit-time enforcement. Agent arguments are proposals; locked ERP records supply business facts. Go PDP performance is supporting evidence. Scope is this one Odoo 17 confirmation path; SAP is applicability discussion only. See [scope and evidence](../../docs/thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) for the shared claim boundary and planned A/B/C evaluation.

## Implemented contract

- Standard generated Protobuf client from `clients/python/v1`.
- Mandatory short-lived HS256 JWT metadata with tenant-bound `sub`.
- Optional mTLS in development/test and mandatory mTLS when
  `PDP_CLIENT_ENV=production`.
- PID-aware gRPC channel recreation after Odoo worker forks.
- CBI-bound delegation proof V2 for protected numeric purchase-order IDs; V1 remains only on the tested legacy non-numeric boundary.
- Transactional nonce ledger unique on
  `(tenant_id, delegation_grant_id, delegation_nonce)`.
- Non-rollback `to approve` transition when a protected `ALLOW` carries `REQUIRE_HUMAN_APPROVAL`; `DENY` cannot be overridden by a later capability.
- Human AC v1 issuance and a bounded approved `button_confirm` route with locked CBI/current-policy rechecks and same-transaction approval/command consumption with the PO mutation.
- Fail-closed rollback on hard deny, configuration error or PDP outage.

## Runtime configuration

### Authority ordering on the protected route

Configure `PDP_ERP_REVOCATION_DATABASE_URL` on **every** PDP and control-plane
writer for the same Odoo database. Besides grant tombstones, this connection
now owns the durable policy-publication barrier. Install/upgrade the addon
before admitting protected traffic; missing tables, stale decision revisions,
pending publication and connection errors fail closed. A fresh first decision
can bootstrap an absent tenant fence; an older Odoo snapshot may need a new
transaction before it can see that row. Never enable a V1 fallback.

The final transaction retains grant, local-authority epoch and tenant-policy
row locks through commit. A local SQL trigger advances the epoch for user,
group/membership, company and grant writes. Policy writers commit `ready=false`
in ERP before mutating PDP policy storage, then publish the committed revision
and `ready=true`. Odoo compares **both** approver and agent evaluated revisions
against the locked row. This is coordinated ordering across two databases,
not a distributed atomic transaction. All policy writers must use the configured
storage APIs; bypass SQL or an unconfigured control plane voids that guarantee.

A deferred constraint checks grant/attempt deadlines and any consumed approval
at PostgreSQL's deferred commit-validation point using its UTC wall clock.
Expiration there aborts the whole PO/approval/command transaction. This does
not promise validity until WAL flush, network acknowledgement or later external
effects. Database clock accuracy and intact triggers are trusted assumptions.
The coarse local epoch trades concurrency for a small, auditable prototype;
its throughput cost is not yet measured.

### Recovering an interrupted policy publication

An error after `ready=false` deliberately leaves protected execution unavailable.
Do not clear the row on timeout, retry with another token, or use a cached ALLOW.
Recovery is an explicit administrative operation, not an automatic startup step:

1. Stop and drain **all** policy/role writers, including in-flight transactions,
   for the affected tenant; verify the configured PDP and ERP database identities.
2. Inspect the pending `publication_id` and the committed PDP policy/role bundle
   and tenant revision. Resolve any failed publication and validate the intended
   committed bundle while protected execution remains fenced.
3. In one ERP transaction, lock that tenant's `pdp_policy_fence_v1` row and compare
   the observed pending token. Set its revision to the verified committed PDP
   revision, `ready=true`, and `publication_id=NULL`; commit. Never reuse a revision
   read while writers can still change it.
4. Require PDP catch-up to that exact revision, verify a scoped decision and ERP
   negative/positive checks, then resume configured writers. Keep an operator
   record of the inspected bundle, token, revision and outcome.

No automated recovery tool or high-availability recovery guarantee is provided.
The recovery procedure itself is operational guidance, not a passed E2E case.

### Client connection settings

The Odoo container must add `clients/python` to `PYTHONPATH` and provide:

| Variable | Purpose |
|---|---|
| `PDP_GRPC_TARGET` | PDP gRPC endpoint. |
| `PDP_JWT_SECRET` | HS256 key trusted by the PDP. |
| `PDP_JWT_ISSUER` | Must equal the PDP `JWT_ISSUER`. |
| `PDP_JWT_AUDIENCE` | Must equal the PDP `JWT_AUDIENCE`. |
| `PDP_DELEGATION_ACTIVE_KID` | Key used to sign new delegation proofs. |
| `PDP_DELEGATION_KEYS_JSON` | Accepted delegation signing key ring. |
| `PDP_CLIENT_ENV` | Set to `production` to require mTLS. |
| `PDP_CLIENT_TLS_CA` | Trusted PDP CA PEM path. |
| `PDP_CLIENT_TLS_CERT` | Odoo client certificate PEM path. |
| `PDP_CLIENT_TLS_KEY` | Odoo client private key PEM path. |
| `PDP_TLS_SERVER_NAME` | Optional certificate server-name override. |

Configure each Odoo company field `PDP Tenant ID` with the exact tenant already
provisioned in the PDP. The environment variable `PDP_TENANT_ID` supplies only
the default for newly created/company-initialized records.

`res.users.PDP Department` is issued as the signed JWT `department` claim, while
`purchase.order.PDP Department` becomes `resource.department`. This keeps the
human same-department policy inputs out of unsigned request principal context.

Secrets must be injected from the deployment secret store. They must never be
stored on delegation-grant records or written to logs.

The PDP server serving this protected route also requires
`PDP_ERP_REVOCATION_DATABASE_URL`, pointing to this exact Odoo database.
The addon creates the private SQL table `pdp_delegation_fence_v1` on install/upgrade.
Every PDP replica accepting revocation for these grants must use this same fence
database. Its database role needs SELECT/INSERT/UPDATE on that table, not access
to purchase orders. The PEP requests and checks the PDP acknowledgement
`odoo-revocation.v1:<database>`; missing/wrong acknowledgement fails closed.

Final transactions lock the tenant/grant fence until commit or rollback. PDP
revocation first commits `revoked=true` there, then publishes its normal durable
revocation. A waiting revoke cannot report success before an earlier final
transaction finishes. If PDP publication fails after the ERP fence commits,
the ERP still denies execution; retry revocation to reconcile the PDP/local grant
display state. A timeout during commit can have an ambiguous result and must be
retried idempotently. Tombstones are never reset by grant reactivation.
This tombstone orders grant revocation. The policy/local-authority fences and
deferred deadline constraint described above cover their separate boundaries;
they do not provide a distributed transaction across both databases.
The testbed selects `odoo_e2e` by default; the Make benchmark
target selects `odoo_orm_benchmark` using `PDP_TESTBED_ERP_DATABASE`.

## Material edits and the final transaction

The delegated line guard locks affected protected purchase-order parents before
ORM line creation, writes (including tax relations), or deletion. It also updates
the parent write version: an existing-line lock alone cannot expose a newly
inserted line to an older PostgreSQL REPEATABLE READ snapshot. Final execution
then retries and reconstructs the changed intent rather than consuming its old
approval. Moving a line includes both old and new parents in sorted lock order.

This protects the repository-owned ORM paths, not arbitrary SQL writers or
extensions bypassing these hooks. It does not prohibit later authorized ERP
edits after confirmation. Ordinary non-delegated parents are not touched by this
guard. Exact exercised schedules and authority-ordering assumptions are recorded in
`docs/technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md` at repo root.

## Verification status

The pure Python protocol tests and the matching Go golden vector verify the
canonical proof bytes. Python syntax, XML parsing and Compose rendering pass.
On 2026-09-12 the isolated Compose runner (`make test-odoo-e2e`) generated
short-lived test certificates, rejected a client without a certificate, then
installed this addon in a fresh Odoo 17 database. All seven real
ORM/mTLS-gRPC/PostgreSQL transaction cases passed with 0 failures/errors,
followed by a passing two-session serialization-retry assertion with one nonce
and one executed attempt.

On 2026-09-24 the expanded fresh-database Odoo/mTLS/PDP/PostgreSQL gate passed
38 post-tests / 48 test cases with 0 failures/errors, including the approved
final route, high-value hard-SoD denial and DENY-with-obligation rejection;
the baseline two-session nonce check also passed. That historical milestone is
superseded by the 2026-09-27 gate: 75 post-tests, zero failures/errors, mTLS,
baseline/approved concurrency and retry, 16 material schedules, three grant
ordering schedules, three committed-authority cases, four post-ALLOW policy/role
schedules and two real-clock expiry rollback cases all pass in one fresh-database
invocation (exit 0). See the EVAL-01 ledger for faults, sources and limits.
This addon is not production-ready.
