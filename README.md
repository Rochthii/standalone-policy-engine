# Standalone Policy Engine

A high-performance, standalone Policy Decision Point (PDP) implementing the PBAC/ABAC authorization model. Written in Go, designed for Cloud-Native microservices architectures requiring sub-millisecond access control decisions under extreme load.

---

## Quick Start

```bash
git clone https://github.com/Rochthii/standalone-policy-engine.git
cd standalone-policy-engine

cp .env.example .env

# Start postgres and redis dependency services
make docker

# Run database migrations
make migrate

# Start the REST Control Plane API
make control-plane

# Start the gRPC PDP Data Plane Server
make pdp
```

---

## Overview

The Policy Engine operates as an independent microservice that decouples authorization logic from application code. Any service in the system — regardless of the underlying database technology — sends a gRPC request to this engine and receives an ALLOW or DENY decision based on declarative policy rules evaluated in-memory.

The system is architecturally equivalent to Open Policy Agent (OPA) or Amazon Verified Permissions (Cedar language), but is purpose-built for multi-tenant SaaS environments where sub-millisecond latency and zero GC pressure are primary design constraints.

### Authorization Flow

```
Client Request
    |
    v
API Gateway (Policy Enforcement Point)
    |
    | gRPC: CheckAccess(tenant, subject, action, resource, context)
    v
Policy Decision Point (This Service)
    |
    | In-memory Trie lookup: O(log N)
    v
AST Evaluator (ABAC expression engine)
    |
    v
Decision: ALLOW / DENY  (avg latency < 0.3ms)
    |
    v
API Gateway enforces decision
```

---

## Architecture

```
Client
   ↓
JWT Auth
   ↓
Control Plane
   ↓
PostgreSQL
   ↓
Redis Pub/Sub
   ↓
PDP Runtime
   ↓
Audit Log
```

### Core Components

**Policy Decision Point — Data Plane** (`cmd/pdp-server`)

- gRPC server exposing `CheckAccess` and `ExplainDecision` endpoints
- Lock-free hot-path: reads use `atomic.LoadPointer`, no mutexes on the critical path
- Copy-On-Write state management for zero-downtime policy updates

**Policy Management API — Control Plane** (`cmd/control-plane`)

- REST API for full policy lifecycle management (create, update, publish, delete)
- `POST /api/v1/tenants/{id}/simulate`: dry-run evaluation against draft policies without affecting the live engine
- `GET /metrics`: Prometheus metrics endpoint

**In-Memory Index (Trie)** (`internal/engine/trie.go`)

- Multi-level index: `TenantID → Subject → Resource → Action`
- Wildcard partitioning separates global rules from tenant-specific rules to prevent index pollution
- Lookup complexity: O(log N) per request

**Role Hierarchy DAG** (`internal/engine/dag.go`)

- Directed Acyclic Graph for role inheritance
- DFS cycle detection at write time
- Transitive closure pre-computed at update time for O(1) membership checks at runtime

**AST Evaluator** (`internal/engine/evaluator.go`)

- Evaluates ABAC condition expressions compiled to typed AST nodes
- Short-circuit evaluation for AND/OR chains
- IP/CIDR matching via pre-compiled uint32 bitmasks (no string parsing at runtime)
- DateTime comparisons via pre-parsed int64 Unix nanoseconds
- `sync.Pool` for evaluation context reuse

**Policy Language Compiler** (`internal/parser/`)

- Cedar-inspired declarative DSL with stateful lexer
- Pratt parser for operator precedence
- Compiler performs constant folding, type checking, and AST depth enforcement (max 15 levels)

---

## Performance Targets & Benchmarks

### Target Targets
| Metric                  | Target                          |
|-------------------------|---------------------------------|
| Average latency         | < 0.3 ms                        |
| P99 latency             | < 1.0 ms under sustained load   |
| Throughput (single core)| > 5,000,000 decisions/second    |
| Memory GC pressure      | Near-zero (sync.Pool + COW)     |
| Policy hot-reload time  | < 300 ms (Redis Pub/Sub)        |

### Actual Benchmark Results
| Metric | Result |
|--------|--------|
| **Decision latency** | 4.433 µs |
| **Throughput** | 2.14M req/s/core |
| **Memory alloc** | 13–39 alloc/op |
| **Isolation** | Multi-tenant |

---

## Project Structure

```
standalone-policy-engine/
├── cmd/
│   ├── pdp-server/          # Data Plane entry point (gRPC)
│   └── control-plane/       # Control Plane entry point (HTTP REST)
├── internal/
│   ├── parser/              # Policy language: Lexer, Parser, AST, Compiler
│   ├── engine/              # Core: Trie index, DAG, Evaluator, Decision, COW, GC
│   ├── server/              # gRPC server, HTTP server, Prometheus metrics
│   ├── storage/             # PostgreSQL persistence, BadgerDB edge store
│   ├── audit/               # Async audit logger with Ring Buffer and Spill-to-Disk
│   └── metrics/             # Prometheus metric definitions
├── proto/v1/                # Protobuf contract: CheckAccess, ExplainDecision
├── deployments/
│   ├── docker/              # Multi-stage Dockerfiles (pdp, control-plane)
│   ├── envoy/               # Envoy L7 proxy configuration for gRPC load balancing
│   └── kubernetes/          # Kubernetes Deployment manifests (PDP + Envoy sidecar)
├── tests/
│   └── performance_test.go  # Go benchmark tests for in-memory decision latency
├── docs/                    # Architecture and design documentation
└── CHANGELOG.md
```

---

## Policy Language

Policies are written in a Cedar-inspired declarative syntax.

**Permit with condition (RBAC + ABAC):**

```
permit(
    principal == user:alice,
    action    == action:DELETE,
    resource  == file:report.pdf
)
when {
    context.ip_address in "10.0.0.0/8" &&
    context.request_time >= "08:00:00Z" &&
    context.request_time <= "18:00:00Z"
};
```

**Explicit deny override:**

```
forbid(
    principal == any,
    action    == any,
    resource  == any
)
when {
    context.device_status == "compromised"
};
```

**Role inheritance:**

```
permit(
    principal in role:admin,
    action    == any,
    resource  in namespace:finance
);
```

### Decision Rules

1. **Deny-by-Default**: if no permit rule matches, the decision is DENY.
2. **Forbid Overrides**: if any forbid rule matches, the decision is DENY regardless of permit rules.
3. **Explicit Permit**: if at least one permit rule matches and no forbid rule matches, the decision is ALLOW.

---

## Infrastructure

### Audit Logging

All access decisions are logged asynchronously via a Ring Buffer (buffered Go channel). A background worker pool performs batch inserts into PostgreSQL using the `pgx` CopyFrom protocol for maximum throughput. If PostgreSQL becomes unavailable, logs are automatically spilled to local SSD as JSON Lines files and replayed into the database when connectivity is restored.

### Cache Synchronization

Policy updates propagate to all PDP nodes via Redis Pub/Sub with a latency target of under 300ms. If the Redis broker is unavailable, each PDP node falls back to polling PostgreSQL every 10 seconds to ensure cache consistency.

### Edge Storage (BadgerDB)

When deployed as a Sidecar, the PDP writes a local snapshot of each tenant's active policy set to an embedded BadgerDB database after every successful synchronization. On restart, if PostgreSQL is not reachable, the engine loads directly from BadgerDB and begins serving requests immediately.

### Tenant Cache GC

The engine tracks the last access time for each tenant's in-memory Trie. A background goroutine runs every hour and evicts Tries for tenants that have been idle for more than 24 hours. When a new request arrives for an evicted tenant, the engine performs a lazy reload from PostgreSQL before serving the decision.

---

## Local Development

### Prerequisites

- Go 1.22 or later
- PostgreSQL 15 or later
- Redis 7 or later

### Environment Variables

| Variable             | Description                                     | Default                                                                     |
|----------------------|-------------------------------------------------|-----------------------------------------------------------------------------|
| `DATABASE_URL`       | PostgreSQL connection string                    | `postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable` |
| `REDIS_URL`          | Redis address                                   | `localhost:6379`                                                            |
| `REDIS_MODE`         | Redis client mode (`single`, `sentinel`, `cluster`) | `single`                                                                    |
| `JWT_SECRET`         | Secret key for validating incoming gRPC JWTs   | `default-policy-engine-super-secret-key-12345`                              |
| `LOG_KEK`            | Key Encrypting Key for AES-GCM log envelope     | `default-log-encryption-key-for-sprint-6-must-be-32-bytes!`                 |
| `GC_ENABLED`         | Enables automatic RAM Trie GC for idle tenants | `true`                                                                      |
| `GC_INTERVAL`        | Interval for running the RAM GC cycle          | `10m` (10 minutes)                                                          |
| `GC_IDLE_TIMEOUT`    | Idle timeout before a tenant is unloaded from RAM| `24h` (24 hours)                                                            |
| `GC_MAX_CACHE_SIZE`  | Maximum number of tenants kept active in RAM    | `1000`                                                                      |

### Database Migrations

The project uses `golang-migrate` to manage the schema database versioning. Migrations are automatically run on startup. Migration files are located in [db/migrations/](file:///e:/Projects/Project_TN/standalone-policy-engine/db/migrations/).

### Running Locally

Start the Data Plane (gRPC, port 50051):

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable"
$env:REDIS_URL    = "localhost:6379"
go run cmd/pdp-server/main.go
```

Start the Control Plane (HTTP, port 8080):

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable"
$env:REDIS_URL    = "localhost:6379"
go run cmd/control-plane/main.go
```

### Running Tests

```bash
# Unit and integration tests
go test -v -race ./internal/...

# Performance benchmark (in-memory decision latency)
go test -bench=. -benchmem ./tests/...
```

---

## API Reference

### gRPC — Data Plane (port 50051)

| Method             | Description                                                          |
|--------------------|----------------------------------------------------------------------|
| `CheckAccess`      | Evaluate a permission request. Returns ALLOW or DENY.                |
| `ExplainDecision`  | Returns the decision and the list of matched policy IDs.             |

### HTTP — Control Plane (port 8080)

| Method | Path                                                         | Description                                        |
|--------|--------------------------------------------------------------|----------------------------------------------------|
| POST   | `/api/v1/tenants/{id}/policies`                              | Create a draft policy.                             |
| PUT    | `/api/v1/tenants/{id}/policies/{policy_id}`                  | Update a draft policy.                             |
| DELETE | `/api/v1/tenants/{id}/policies/{policy_id}`                  | Delete a policy.                                   |
| POST   | `/api/v1/tenants/{id}/policies/{policy_id}/publish`          | Compile, validate, and activate a policy.          |
| POST   | `/api/v1/tenants/{id}/simulate`                              | Dry-run evaluation against draft policies.         |
| POST   | `/api/v1/decisions`                                          | REST fallback for CheckAccess.                     |
| POST   | `/api/v1/decisions/explain`                                  | REST fallback for ExplainDecision.                 |
| GET    | `/metrics`                                                   | Prometheus metrics.                                |

---

## Deployment

### Docker

```bash
# PDP Server
docker build -f deployments/docker/Dockerfile.pdp -t policy-engine/pdp:latest .

# Control Plane
docker build -f deployments/docker/Dockerfile.control -t policy-engine/control-plane:latest .
```

### Kubernetes

The PDP is deployed as a 3-replica Deployment with an Envoy proxy sidecar container in each pod. Envoy handles L7 gRPC load balancing and HTTP/2 connection multiplexing so that upstream clients do not need to manage persistent connections manually.

```bash
kubectl apply -f deployments/kubernetes/
```

---

## Design Decisions

| Decision                              | Rationale                                                                                         |
|---------------------------------------|---------------------------------------------------------------------------------------------------|
| Atomic pointer swap (COW)             | Allows lock-free reads on the hot path while writes atomically replace the entire state snapshot. |
| Trie-based index (not linear scan)    | O(log N) lookup per request. Linear scan over all policies is prohibited by design.              |
| Transitive closure in DAG             | Pre-computed at write time so role membership checks at runtime are O(1).                         |
| AST depth limit (15 levels)           | Prevents stack overflow from deeply nested expressions used as denial-of-service vectors.          |
| Fail-closed on missing attributes     | Missing context attributes resolve to a typed error that evaluates to false, not to a default.   |
| BadgerDB as edge store                | Embedded, dependency-free local persistence for zero-downtime cold starts in sidecar deployments.|

---

## Command Line Tool (`pectl`)

A production-grade command-line tool named **`pectl`** is provided for interacting with the Control Plane REST API.

### Build and Install

```bash
# Setup dependencies and build using script
# Windows (PowerShell)
.\scripts\setup-pectl.ps1

# Linux/macOS/WSL (Bash)
bash scripts/setup-pectl.sh

# Or build via Makefile
make tidy
make build-pectl
```

The compiled binary will be placed at `bin/pectl` (or `bin/pectl.exe`).

### Configuration

`pectl` can be configured via flags, environment variables, or a config file (loaded in order of priority: Flags > Environment > Config File).

Configuration file path: `~/.pectl/config.yaml`
```yaml
server: http://localhost:8080
auth:
  token: your-jwt-token
output: table
timeout: 10s
```

Environment variables:
- `PECTL_SERVER`
- `PECTL_TOKEN`
- `PECTL_OUTPUT`

### Example Usage

```bash
# Verify version
pectl version

# List all policies for a tenant
pectl policy list tenant-123

# Create a draft policy
pectl policy create tenant-123 --effect permit --file policy.cedar

# Publish policy to engine memory
pectl policy publish tenant-123 policy-uuid-xxx

# Run evaluation simulation
pectl simulate tenant-123 --subject user:alice --action read --resource file:doc --context-file ctx.json --draft-file draft.cedar

# Perform dynamic permission checks
pectl check tenant-123 --subject user:alice --action read --resource file:doc

# Check metrics and health
pectl metrics
pectl health
```

---

## License

This project is for research and educational purposes as part of the Standalone Authorization Engine design study.
