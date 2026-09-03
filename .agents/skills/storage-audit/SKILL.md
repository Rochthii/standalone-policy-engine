---
name: storage-audit
description: Expert rules for PostgreSQL pgx.CopyFrom, Async Audit Ring Buffer, Vector Sidecar, and Postgres Sync.
---

# Storage, Cache Sync & Audit Logging Skill

## 🎯 Mission
Ensure immutable audit logging without blocking the hot path (< 140ns) and provide gap-detecting policy synchronization (< 50ms) using pure PostgreSQL (Zero Redis).

## 🔑 Critical Implementation Rules
1. **Pure PostgreSQL Monotonic Sequence (Zero Redis)**:
   - Atomic sequence: `UPDATE tenants SET revision = revision + 1 ... RETURNING revision`.
   - Emits `NOTIFY policy_events, '{"tenant_id":"...", "revision":..., "action":"UPDATE"}'` (< 120 bytes, strictly under 8000-byte limit).
   - In-memory `ReplayBuffer` on Control Plane serves missed events without disk I/O.
   - Gap detection: If `event.Revision > current + 1`, trigger instant Fast Catch-Up (< 50ms) without waiting for 10s polling.

2. **Async Audit Ring Buffer & WORM Architecture**:
   - Channel-based lock-free queue (`chan *LogEntry`, buffer capacity 5000+).
   - Batch insert via `pgx.CopyFrom` directly into `decision_audit_logs`.
   - Spill-to-Disk: On DB failure or backpressure, spill JSON lines to `./spill-logs` (1GB quota), replaying on recovery.
   - WORM Encryption: AES-GCM envelope encryption using `LOG_KEK`.

3. **Edge Embedded Profile**:
   - BadgerDB local KV store in `./badger-data` for cold-start and air-gapped IoT gateways when PostgreSQL is unreachable.

## 📂 Source Files
- [`internal/audit/logger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go)
- [`internal/storage/postgres.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/postgres.go)
- [`internal/storage/badger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger.go)
- [`internal/engine/sync.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/sync.go)
- [`internal/server/replay_buffer.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/replay_buffer.go)
