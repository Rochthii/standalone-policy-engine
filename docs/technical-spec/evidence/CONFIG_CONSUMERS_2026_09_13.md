# OPS-CONFIG-01 Evidence — 2026-09-13

`TestConfigRuntimeEnvironmentBindings` sets every PDP/Control Plane runtime input and proves it reaches the centralized `config.Config` consumed by the two process entrypoints.

| Group | Bound inputs | Runtime consumer |
|---|---|---|
| Server | `HTTP_PORT`, `GRPC_PORT`, limits, listener socket, Ziti inputs | Control Plane HTTP server; PDP gRPC listener and transport |
| Database/engine | database URL, storage mode, Badger and GC/sync controls | PostgreSQL storage, engine GC and syncer |
| Audit | queue, batch, timing and spill controls | encrypted batch logger and spill store |
| Security | JWT, delegation/audit key rings and TLS files | gRPC/REST validators, delegation manager, audit crypto and mTLS |

Removed inputs: `REDIS_URL`, `AUDIT_SOCKET_PATH`, `APP_PORT`, `PORT`, `METRICS_PORT` and `LOG_LEVEL` had no production consumer. `ZITI_IDENTITY_PATH` and `ZITI_SERVICE_NAME` are no longer read directly by the PDP entrypoint.

Executed: `go test -count=1 ./internal/config ./cmd/pdp-server` and `docker compose -f docker-compose.testbed.yml config --quiet` both passed.
