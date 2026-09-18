# OPS-DEPLOY-06 — Container runtime hardening evidence — 2026-09-18

## Result

Commit `4fee058594f48076bc836d0eb79db74ddb92c6c7` hardens the PDP and Control
Plane runtime images and Kubernetes templates.

- Both runtime images create and select the unprivileged `pdp` user (UID/GID
  `10001`).
- The PDP audit spill directory is the only application directory owned by
  that user; the previous world-writable (`0777`) directory was removed.
- Kubernetes templates set `runAsNonRoot`, `RuntimeDefault` seccomp,
  `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, and drop
  all Linux capabilities. Writable `emptyDir` mounts are explicit for `/tmp`
  and the PDP spill directory.

## Automated checks

```powershell
docker build --tag standalone-pdp-hardening:local --file deployments/docker/Dockerfile.pdp .
docker build --tag standalone-control-hardening:local --file deployments/docker/Dockerfile.control .
docker run --rm --read-only --cap-drop ALL --security-opt no-new-privileges `
  --entrypoint sh standalone-pdp-hardening:local -c 'id; test -w /app/spill-logs; test ! -w /app'
docker run --rm --read-only --cap-drop ALL --security-opt no-new-privileges `
  --entrypoint sh standalone-control-hardening:local -c 'id; test ! -w /app'
docker compose -f docker-compose.testbed.yml config --quiet
git diff --check
```

All commands passed. Both runtime probes reported
`uid=10001(pdp) gid=10001(pdp)`.

## Boundary

No live Kubernetes API server was configured on the validation host, so
`kubectl` could not perform admission validation. This evidence proves the
image runtime restrictions directly and records the corresponding Kubernetes
template configuration; it does not claim a deployed-cluster smoke test.
