# Role-Inheritance Resilience Evidence — 2026-09-13

## Scope

`CONS-ROLE-05` requires a tenant's role-inheritance graph to persist with its revision, rebuild into the engine after fresh process state, and be replaced when reconciliation discovers a missed revision event.

## Environment and commands

Each integration test creates and removes its own PostgreSQL 15 database under the repository Docker testbed.

```powershell
$env:TEST_DATABASE_URL='<testbed PostgreSQL URL>'
go test -count=1 -v ./internal/engine -run TestSyncerRestartsAndReconcilesRoleInheritance
go test -count=1 -v ./internal/storage -run TestStorage_MigrationsIntegration
```

Result: both commands passed on 2026-09-13.

## Evidence

1. A role graph (`user:alice -> role:manager -> role:staff`) is persisted through `ReplaceRoleInheritances`, which advances the tenant revision in PostgreSQL.
2. A fresh `EngineWithGC` loads the real PostgreSQL policy bundle and rebuilds a DAG in which `user:alice` inherits `role:staff`.
3. The role graph is replaced in PostgreSQL with `user:alice -> role:auditor`; no listener callback is delivered to the engine.
4. `reconcileTenantRevision` reads the newer revision and real bundle, atomically swaps the engine tenant state, adds `role:auditor`, and removes the stale `role:staff` relationship.

The test exercises a fresh in-memory engine rather than an operating-system process restart; it verifies the same storage-to-engine bootstrap path without claiming process-supervision coverage.
