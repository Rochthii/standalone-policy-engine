# OPS-HEALTH-03 — PDP readiness evidence — 2026-09-17

## Scope

This change distinguishes process liveness from authorization readiness for the PDP and control plane. It does not claim a production Kubernetes deployment, an Odoo transaction run, or overall production readiness.

## Runtime contract

| Endpoint | Success | Failure behavior |
|---|---|---|
| `GET /livez` | `200` if the probe server is running | Does not depend on PostgreSQL or policy synchronization. |
| `GET /readyz` | `200` only when PostgreSQL responds, the `LISTEN policy_events` session is active, and every loaded tenant's in-memory revision matches PostgreSQL | `503` with `degraded` or `not_ready`; Kubernetes removes the pod from ready endpoints. |
| `GET /api/v1/health` | Same readiness report, for `pectl health` | Same `503` behavior. |

`not_ready` is the startup state until the policy listener has successfully executed `LISTEN`. A listener/revision error is `degraded`. A failed PostgreSQL ping is `not_ready` because authorization policy state cannot be trusted.

Odoo is downstream of PDP. The Compose Odoo services now wait for PDP's healthcheck instead of merely a started container, so a degraded PDP is not presented to Odoo as available. PDP deliberately does not probe Odoo, which would create a circular readiness dependency.

## Automated checks

```powershell
go test -count=1 ./internal/engine ./internal/server ./internal/storage
docker compose -f docker-compose.testbed.yml config --quiet
docker compose -f tests/docker-compose.yml config --quiet
```

Result on this change: all commands above passed. `go vet ./...` also passed.

The Go test covers `healthy`, policy-sync `degraded`, startup `not_ready`, PostgreSQL failure, independent liveness, and loaded-tenant revision lag. The two Compose validations ensure the healthchecks and Odoo dependency conditions are syntactically valid. Kubernetes manifests use `/readyz` for readiness and `/livez` for liveness; a live Kubernetes API server was not available for a cluster admission run.
