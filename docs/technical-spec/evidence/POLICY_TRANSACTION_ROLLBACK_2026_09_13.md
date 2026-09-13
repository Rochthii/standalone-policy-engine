# Policy Transaction Rollback Evidence — 2026-09-13

## Scope

`CONS-REV-03` requires policy publish, ACTIVE-to-DRAFT update, and delete to remain atomic with tenant revision allocation and policy event notification.

## Environment and command

An isolated, test-owned database was created under the repository PostgreSQL 15 Docker testbed. The test was executed with:

```powershell
$env:TEST_DATABASE_URL='<testbed PostgreSQL URL>'
go test -count=1 -v ./internal/storage -run TestStoragePolicyMutationsRollbackOnPostgresFaults
```

Result: `PASS` on 2026-09-13.

## Cases

| Fault at transactional boundary | Publish | ACTIVE-to-DRAFT update | Delete |
|---|---:|---:|---:|
| PostgreSQL `BEFORE UPDATE OF revision` trigger raises an exception | PASS | PASS | PASS |
| Notifier executes an injected PostgreSQL SQL error in the same `pgx.Tx` | PASS | PASS | PASS |

Each case asserts that the original policy text, status, compiled AST, version, and tenant revision match the pre-mutation baseline after the operation returns an error.

## Limitation

The production notifier is PostgreSQL `pg_notify`. PostgreSQL does not provide a practical fault switch for a normal, fixed-size `pg_notify` payload in this isolated testbed. The second fault therefore injects a real PostgreSQL execution error at the notifier boundary in the same transaction; it proves the rollback behavior of the production error path without claiming to induce an internal `pg_notify` failure.
