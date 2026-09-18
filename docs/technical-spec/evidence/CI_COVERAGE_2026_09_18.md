# CI critical coverage gate — 2026-09-18

## Measured baseline

On commit `db60aaf`, the targeted command below passed with statement coverage
of server 49.4%, storage 22.4% and security 74.1%:

```bash
go test -count=1 -cover ./internal/server ./internal/storage ./internal/security
```

## Gate

The `Unit Tests` job now enforces minimum statement coverage of 45% for
`internal/server`, 20% for `internal/storage`, and 70% for
`internal/security`. These are deliberately below the measured baseline to
allow normal toolchain variance while preventing a material regression.

The job also names the critical fail-closed server and security tests. The
PostgreSQL job runs the storage migration, transaction-rollback and
three-replica revocation tests against its real service database.

Remote CI verification is required before the G9 item can be marked passed.
