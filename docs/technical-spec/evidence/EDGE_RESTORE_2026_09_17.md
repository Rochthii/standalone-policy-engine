# EDGE-RESTORE-05 — Badger edge policy restore evidence — 2026-09-17

## Scope

`STORAGE_MODE=edge` restores cached policy state without opening PostgreSQL. This is a constrained offline policy-evaluation profile, not production ERP readiness: PostgreSQL audit delivery and durable revocation synchronization are unavailable. Delegated requests therefore fail closed.

## Restore contract

- Cloud policy synchronization persists a versioned Badger snapshot containing stable policy IDs, DSL source, role inheritance and tenant revision.
- Edge startup lists every snapshot, validates its format, recompiles the DSL, rebuilds each Trie/DAG and atomically publishes the complete state before opening the gRPC listener.
- No snapshot, legacy/invalid snapshot, invalid DSL or invalid role graph aborts startup; no partial in-memory state is published.

## Automated check

```powershell
go test -count=1 ./internal/storage ./internal/engine ./internal/server
go test -run=^$ ./cmd/pdp-server
```

`TestRestoreEdgeSnapshotsAfterOfflineRestart` writes a real Badger snapshot, closes the store, reopens it without PostgreSQL, restores a fresh engine, verifies its persisted revision and evaluates an `ALLOW` decision. `TestRestoreEdgeSnapshotsFailsClosedWithoutSnapshot` proves an empty store cannot publish state. The storage tests reject incomplete snapshots.
