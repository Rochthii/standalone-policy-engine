# Odoo 17 PDP Authorizer

This addon is the repository-owned Policy Enforcement Point (PEP) for purchase
order confirmation. It replaces the incompatible Project 2 prototype.

## Implemented contract

- Standard generated Protobuf client from `clients/python/v1`.
- Mandatory short-lived HS256 JWT metadata with tenant-bound `sub`.
- Optional mTLS in development/test and mandatory mTLS when
  `PDP_CLIENT_ENV=production`.
- PID-aware gRPC channel recreation after Odoo worker forks.
- Full delegation proof `v1.<kid>.<hmac>` matching the Go canonical encoding.
- Transactional nonce ledger unique on
  `(tenant_id, delegation_grant_id, delegation_nonce)`.
- Non-rollback `to approve` transition for `REQUIRE_HUMAN_APPROVAL`.
- Fail-closed rollback on hard deny, configuration error or PDP outage.

## Runtime configuration

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

## Verification status

The pure Python protocol tests and the matching Go golden vector verify the
canonical proof bytes. Python syntax, XML parsing and Compose rendering pass.
On 2026-09-12 the isolated Compose runner (`make test-odoo-e2e`) generated
short-lived test certificates, rejected a client without a certificate, then
installed this addon in a fresh Odoo 17 database. All seven real
ORM/mTLS-gRPC/PostgreSQL transaction cases passed with 0 failures/errors,
followed by a passing two-session serialization-retry assertion with one nonce
and one executed attempt.
