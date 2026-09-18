# OPS-RUNBOOK-07 — Recovery and incident rehearsal evidence — 2026-09-18

## Result

The recovery procedures in commit
`e0db6aba928d7744d8a1660152c30f893492d668` were rehearsed without using a
user or production database.

| Procedure | Executed evidence | Result |
|---|---|---|
| PostgreSQL backup and restore | A uniquely named, digest-pinned PostgreSQL 15 container created source data, wrote a custom-format `pg_dump`, restored it into a second database, and read the sentinel value `restore-verified`. | PASS |
| Policy rollback | `go test -count=1 ./internal/storage -run '^TestStoragePolicyMutationsRollbackOnPostgresFaults$'` against the isolated PostgreSQL container. | PASS |
| Audit key rotation/replay | `go test -count=1 ./internal/audit -run '^TestAuditSpillReplayAcrossRestartAndKeyRotation$'`. | PASS |
| Authorization incident | `go test -count=1 ./internal/server -run '^(TestReadinessEndpoint|TestLivenessEndpointDoesNotDependOnPostgreSQLOrSync)$'`. | PASS |

The temporary PostgreSQL container used an automatically assigned loopback port
and was stopped with Docker's `--rm` cleanup after the rehearsal. No test
database or container remained afterward.

## Boundary

This is an isolated local rehearsal. It does not claim a production database
restore, live Kubernetes incident exercise, external WORM archival, KMS/HSM
rotation, or deletion-proof compliance. Those remain separate release evidence
requirements.
