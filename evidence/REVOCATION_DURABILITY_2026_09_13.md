# Durable Multi-Replica Revocation Evidence

> Date: 2026-09-13
> Gate: `REV-DURABLE-02` / G4

## Implemented boundary

- PostgreSQL `delegation_revocations` is the durable source of truth.
- `RevokeDelegation` returns success only after the row and `pg_notify` commit.
- Each PDP establishes `LISTEN`, loads the active snapshot, then becomes ready.
- Reconnect repeats the snapshot-first sequence; duplicate delivery is idempotent.
- Delegated checks fail closed while revocation state is unavailable.
- The authorization hot path remains an in-memory tenant/grant lookup.

The documented healthy-path propagation deadline is 5 seconds.

## Automated evidence

Command:

```powershell
$env:TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable'
go test -v -count=3 -timeout=90s ./internal/storage -run='^TestRevocationPropagationThreeReplicasAndRestart$'
```

Environment: PostgreSQL 15 Alpine in an isolated Docker Compose project on Windows/amd64.

The test uses three independent managers/listeners, concurrent checks, twelve revokes, a deliberate 25ms delivery delay on one replica, one offline replica, and a fresh-manager restart from the durable snapshot.

| Run | Samples | p50 | p95 | p99 | Max |
|---:|---:|---:|---:|---:|---:|
| 1 | 36 | 9.3135ms | 36.0194ms | 36.8369ms | 38.2697ms |
| 2 | 36 | 7.3413ms | 32.5403ms | 33.7159ms | 37.3222ms |
| 3 | 36 | 7.4655ms | 32.2648ms | 32.5222ms | 38.8256ms |

Result: all three runs passed. The worst observed propagation was 38.8256ms, including the injected delay. Restart restored both a revocation committed while the replica was offline and earlier active revocations.

Additional gates:

- `go test -count=1 ./...` with PostgreSQL integration: PASS.
- `go vet ./...`: PASS.
- Local race detector: unavailable because local CGO is disabled; tracked separately by `CI-003` and G9.

## Scope

This evidence verifies the repository's PostgreSQL-backed revocation path and three in-process PDP replica caches against one real PostgreSQL instance. It does not claim cross-region behavior or a remote CI result.
