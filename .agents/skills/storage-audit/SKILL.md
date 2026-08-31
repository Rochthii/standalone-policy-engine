---
name: storage-audit
description: Expert rules and constraints for PostgreSQL pgx.CopyFrom, Async Audit Ring Buffer, Spill-to-Disk, BadgerDB, and Redis synchronization.
---

# Storage, Cache Sync & Audit Logging Skill

## 🎯 Mission
Ensure immutable audit logging without blocking the hot path, and provide multi-tiered fault-tolerant policy synchronization.

## 🔑 Critical Implementation Rules
1. **Asynchronous Audit Logger**:
   - **Lock-free Ring Buffer**: Channel-based queue (`chan *LogEntry`, buffer capacity 5000+).
   - **Batch Insert**: Stream directly to PostgreSQL `decision_audit_logs` using `pgx.CopyFrom`.
   - **Spill-to-Disk**: If PostgreSQL write fails or buffer fills, spill raw JSON Lines to SSD (`./spill-logs`). Automatically replay on DB recovery with backoff.
   - **WORM Encryption**: Support AES-GCM envelope encryption with Key Encrypting Key (`LOG_KEK`).
2. **PostgreSQL Persistence & Migrations**:
   - Automated schema migrations via `golang-migrate` on startup from `db/migrations/`.
   - `policies` table stores raw `policy_text`, compiled `ast_json`, and incrementing `version`.
3. **Multi-Tier Cache Sync**:
   - **Redis Pub/Sub**: Channel `policy-updates` triggers tenant re-compile and COW swap in $< 300$ms.
   - **Polling Fallback**: Background worker polls PostgreSQL every 10s if Redis is unreachable.
   - **BadgerDB Edge Store**: Embedded local snapshot in `./badger-data` for cold-start when DB is down.

## 📂 Source Files
- [`internal/audit/logger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go)
- [`internal/storage/postgres.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/postgres.go)
- [`internal/storage/badger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger.go)
- [`internal/engine/sync.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/sync.go)
- [`db/migrations/`](file:///e:/Projects/Project_TN/standalone-policy-engine/db/migrations/)
