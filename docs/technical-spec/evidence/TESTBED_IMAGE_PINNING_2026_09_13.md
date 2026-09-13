# Testbed Image Pinning Evidence — 2026-09-13

## Scope

`OPS-IMAGE-02` freezes the external image provenance used by `docker-compose.testbed.yml`. Repository-local PDP and Odoo images remain Compose build outputs; all of their external bases are pinned.

## Pinned external images

| Image use | Reference |
|---|---|
| PostgreSQL testbed database | `postgres:15-alpine@sha256:fe0737ba566a2c5b2a28f34433c0a423261900ec17b9bf7ad115e1aae7e57f1b` |
| Go PDP builder and core runner | `golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59` |
| PDP/control runtime | `alpine:3.19@sha256:6baf43584bcb78f2e5847d1de515f23499913ac9f12bdf834811a3145eb11ca1` |
| Odoo services | `odoo:17.0@sha256:8779f8157ddcc37b84014c94646073f46fd4ad776b7a5d95fc3b0dc73a24a8fa` |

Go 1.25 is selected because `go.mod` declares `go 1.25.0`; the older 1.22 example in the Docker skill is not compatible with this module requirement.

## Self-contained boundary

All Compose build contexts, bind mounts, addon sources, generated Python client, test scripts and database initialization files resolve within this repository. No external filesystem directory is referenced.

## Verification

```powershell
docker compose -f docker-compose.testbed.yml --profile e2e config --quiet
docker compose -f docker-compose.testbed.yml --profile e2e run --build --rm testbed-certgen
docker compose -f docker-compose.testbed.yml --profile e2e up --build --abort-on-container-exit --exit-code-from testbed-odoo-e2e testbed-odoo-e2e
```

Result: PASS on 2026-09-13. The build resolved every listed digest, generated short-lived mTLS certificates, and completed seven Odoo transaction tests plus the two-session nonce concurrency check with exit code 0.

## Limitation

Digest pinning freezes image provenance, not future vulnerability status. Updating a base image remains an explicit review, repin and E2E-verification operation.
