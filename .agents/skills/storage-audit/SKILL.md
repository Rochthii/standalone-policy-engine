---
name: storage-audit
description: Expert rules and constraints for PostgreSQL pgx.CopyFrom, Async Audit Ring Buffer, Vector Sidecar UDS, BadgerDB, and Postgres Monotonic Sequence Sync.
---

# Storage, Cache Sync & Audit Logging Skill

## 🎯 Mission
Ensure immutable audit logging without blocking the hot path (< 140ns), eliminate external sync dependencies like Redis, and provide fault-tolerant, gap-detecting policy synchronization with sub-50ms propagation.

## 🔑 Critical Implementation Rules

### 1. Pure PostgreSQL Sequence & Distributed Sync (Zero Redis, Zero 10s Polling)
- **Monotonic Revision Sequence**:
  - PostgreSQL transaction atomically increments `tenants.revision` (`UPDATE tenants SET revision = revision + 1 WHERE id = ... RETURNING revision`).
  - Emits `NOTIFY policy_events, '{"tenant_id":"...", "revision":..., "action":"UPDATE"}'`.
- **Strict 8000-Byte Postgres Limit Rule**:
  - `NOTIFY` payload MUST NEVER contain full policy AST or policy bodies.
  - ONLY emit metadata (< 120 bytes) containing `tenant_id`, `revision`, and `action`.
- **Replay Ring Buffer on Control Plane**:
  - Fixed in-memory ring buffer (`internal/server/replay_buffer.go`) holds the last $N$ events (e.g. 1000 items) to answer `GET /api/v1/tenants/{id}/events?since_revision=X` instantly without hitting disk.
- **Monotonic Gap Detection & Instant Catch-Up (< 50ms)**:
  - PDP Data Plane tracks `lastKnownRevision`.
  - When receiving an event with `ev.Revision > lastKnownRevision + 1` (gap detected), immediately trigger Fast Catch-Up sync via HTTP/gRPC without waiting for any polling interval.
  - Reconnect handshake automatically syncs missed events and reloads state atomically (COW swap).

### 2. High-Performance Audit Logging & Vector Sidecar
- **Zero-Alloc Hot-Path UDS Datagram**:
  - Non-blocking UDP over Unix Domain Socket (`/var/run/vector/audit.sock`) allows fire-and-forget logging to Vector sidecar without socket backpressure on Go runtime.
- **Async DB Ring Buffer & WORM Architecture**:
  - Channel-based lock-free queue (`chan *LogEntry`, buffer capacity 5000+).
  - Batch insert directly into PostgreSQL `decision_audit_logs` using `pgx.CopyFrom`.
  - **Spill-to-Disk**: If PostgreSQL write fails or buffer fills, spill raw JSON Lines to SSD (`./spill-logs`, 1GB quota). Automatically replay on DB recovery with backoff.
  - **WORM Envelope Encryption**: AES-GCM envelope encryption with Key Encrypting Key (`LOG_KEK`).

### 3. Edge Embedded Storage Profile
- **BadgerDB Edge Store**: Embedded local KV store in `./badger-data` for cold-start and air-gapped IoT gateways when PostgreSQL is unreachable.

## 📂 Source Files
- [`internal/audit/logger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go)
- [`internal/storage/postgres.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/postgres.go)
- [`internal/storage/badger.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger.go)
- [`internal/engine/sync.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/sync.go)
- [`internal/server/replay_buffer.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/replay_buffer.go)
- [`db/migrations/`](file:///e:/Projects/Project_TN/standalone-policy-engine/db/migrations/)
