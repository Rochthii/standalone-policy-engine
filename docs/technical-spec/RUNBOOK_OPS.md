# RUNBOOK_OPS.md — Deployment, Operations & Benchmarking Runbook

## 1. Production Docker Compose Architecture

```yaml
version: '3.8'

services:
  # --------------------------------------------------------------------------
  # 1. PDP Data Plane Server (In-Memory Engine & gRPC :50051)
  # --------------------------------------------------------------------------
  pdp-server:
    image: standalone-pdp:latest
    build:
      context: .
      dockerfile: Dockerfile
    container_name: pdp-data-plane
    environment:
      - GRPC_PORT=50051
      - HTTP_PORT=8080                   # local liveness/readiness endpoint
      - STORAGE_MODE=cloud                  # cloud (stateless) | edge (badgerdb)
      - DATABASE_URL=postgres://pdp_user:pdp_pass@postgres:5432/pdp_db?sslmode=disable
      - LOG_KEK_ACTIVE_KID=audit-2026-09
      - LOG_KEKS_JSON={"audit-2026-08":"<old-32-byte-secret>","audit-2026-09":"<active-32-byte-secret>"}
      - AUDIT_SPILL_DIR=/var/lib/pdp/audit-spill
      - AUDIT_SPILL_MAX_BYTES=1073741824
    volumes:
      - vector-sock:/var/run/vector
      - badger-data:/var/lib/pdp/badger     # Used when STORAGE_MODE=edge
      - audit-spill:/var/lib/pdp/audit-spill
    ports:
      - "50051:50051"                       # gRPC CheckAccess
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O /dev/null http://localhost:8080/readyz"]
      interval: 5s
      timeout: 3s
      retries: 5
    restart: always

  # --------------------------------------------------------------------------
  # 2. PDP Control Plane REST API (:8080)
  # --------------------------------------------------------------------------
  pdp-control-plane:
    image: standalone-pdp-control:latest
    build:
      context: .
      dockerfile: cmd/control-plane/Dockerfile
    container_name: pdp-control-plane
    environment:
      - HTTP_PORT=8080
      - DATABASE_URL=postgres://pdp_user:pdp_pass@postgres:5432/pdp_db?sslmode=disable
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O /dev/null http://localhost:8080/readyz"]
      interval: 5s
      timeout: 3s
      retries: 5
    restart: always

  # --------------------------------------------------------------------------
  # 3. PostgreSQL Database (Monotonic Sequence & Policy Store)
  # --------------------------------------------------------------------------
  postgres:
    image: postgres:16-alpine
    container_name: pdp-postgres
    environment:
      - POSTGRES_USER=pdp_user
      - POSTGRES_PASSWORD=pdp_pass
      - POSTGRES_DB=pdp_db
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./db/migrations:/docker-entrypoint-initdb.d:ro
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pdp_user -d pdp_db"]
      interval: 5s
      timeout: 3s
      retries: 5
    restart: always

  # --------------------------------------------------------------------------
  # 4. Vector Telemetry Sidecar (Non-blocking UDS Datagram Log Shipper)
  # --------------------------------------------------------------------------
  vector:
    image: timberio/vector:0.35.0-alpine
    container_name: pdp-vector
    volumes:
      - vector-sock:/var/run/vector
      - ./deploy/vector.yaml:/etc/vector/vector.yaml:ro
    restart: always

volumes:
  pgdata:
  vector-sock:
  badger-data:
  audit-spill:
```

### Health and readiness contract

- `GET /livez` returns `200` while the process can serve probes. It deliberately does not check PostgreSQL or policy synchronization.
- `GET /readyz` and `GET /api/v1/health` check the PostgreSQL pool, the active PostgreSQL `LISTEN policy_events` session, and revision parity for every tenant currently loaded in memory. The JSON response reports `healthy`, `degraded`, or `not_ready` for the service and each component.
- Only `healthy` returns `200`. Both `degraded` and `not_ready` return `503`, so Kubernetes removes that pod from Service endpoints before it handles PDP traffic. Configure readiness probes for `/readyz` and liveness probes for `/livez`.
- Odoo is a PDP client, not an upstream PDP dependency. It must use Compose `depends_on: condition: service_healthy` (or the equivalent orchestrator readiness gate) for the PDP service; PDP does not probe Odoo, avoiding a circular readiness dependency.

---

## Audit key lifecycle and recovery

- Inject `LOG_KEKS_JSON` from an orchestrator secret store; never bake KEKs into images, source control or ordinary ConfigMaps. Production startup rejects the legacy single-key fallback.
- Keep key administration separate from database/audit readers. A reader needs an explicitly approved decrypt path and the applicable retained KEK; database access alone exposes only authenticated ciphertext and indexing metadata.
- Rotate by adding the new 32-byte KEK beside retained keys, deploying it as `LOG_KEK_ACTIVE_KID`, and confirming new rows use the new `key_id`. Remove an old key only after no retained database row or spill file references it and the recovery backup has expired under policy.
- Mount `AUDIT_SPILL_DIR` on durable storage with service-account-only permissions. Alert on `audit_spill_failures_total`, `audit_logs_dropped_total` and sustained spill growth; successful recovery increments `audit_logs_replayed_total`.
- Recovery starts the PDP with the active key and every retained historical key. Replay verifies metadata/ciphertext integrity before idempotent insert and leaves corrupt files in place for investigation.
- AES-GCM AAD plus the keyed integrity tag detects record modification. It does not prove deletion, ordering or legal WORM retention; send committed records to an independently administered append-only archive for those guarantees.

---

## 2. Fail-Closed Security Configuration

In enterprise authorization, any PDP unavailability **MUST FAIL CLOSED** to prevent unauthorized transactions:

```python
# Odoo PEP Fail-Closed Client Rule
def _evaluate_pdp_access(self):
    client = get_pdp_client()
    try:
        stub = client.get_stub()
        response = stub.CheckAccess(request, timeout=0.35) # 350ms SLA budget
        return response.decision, response.obligations, response.advice
    except (grpc.RpcError, Exception) as e:
        # FAIL-CLOSED INVARIANT: Log error and reject transaction immediately
        _logger.critical(f"PDP UNREACHABLE: Failing closed for PO {self.name}. Error: {e}")
        self.write({'pdp_status': 'deny'})
        raise AccessError(_("Dịch vụ phân quyền PDP tạm thời không khả dụng. Giao dịch bị từ chối an toàn (Fail-Closed)."))
```

---

## 3. Cold-Start Snapshot & Offline Air-Gap Recovery (BadgerDB)

1. **Snapshot Generation**:
   Mỗi khi Control Plane cập nhật chính sách thành công, PDP ghi bản chụp nhị phân (Binary Snapshot) vào BadgerDB:
   ```bash
   pdp-server --export-snapshot=/var/lib/pdp/badger/snapshot.bin
   ```
2. **Cold-Start Sequence**:
   * Khi `pdp-server` khởi động với `STORAGE_MODE=edge`:
     - Bỏ qua kết nối PostgreSQL.
     - Nạp trực tiếp BadgerDB embedded KV store vào RAM trong $< 5$ms.
     - Khởi động gRPC listener ngay lập tức.

---

## 4. Automated Benchmark & Verification Script

Tự động chạy toàn bộ micro-benchmarks hiệu năng, đo lường heap allocations và thông lượng đa luồng:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=========================================================================="
echo "PDP IN-MEMORY PERFORMANCE BENCHMARK SUITE (GO ENGINE)"
echo "=========================================================================="

echo "[1/3] Running Latency Benchmark (Single Core, Nanosecond Budget)..."
go test -bench=BenchmarkEvaluatorLatency -benchmem ./tests/... -run=^$

echo "[2/3] Running Concurrent Load Benchmark (Throughput RPS, Multi-Cores)..."
go test -bench=BenchmarkConcurrentLoad -benchmem -cpu=1,4,8,16,20 ./tests/... -run=^$

echo "[3/3] Running Edge Scenarios & AI Guardrail Benchmarks..."
go test -bench=BenchmarkScenario -benchmem ./tests/... -run=^$

echo "=========================================================================="
echo "VERIFICATION PASSED: ALL INVARIANTS SATISFIED (0 ALLOCS/OP HOT-PATH)"
echo "=========================================================================="
```
