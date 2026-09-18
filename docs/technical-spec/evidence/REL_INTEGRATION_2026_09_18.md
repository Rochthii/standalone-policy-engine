# Combined Control Plane E2E evidence — 2026-09-18

## Gate

The dedicated Compose fixture starts a fresh digest-pinned PostgreSQL 15
database, the Control Plane and the PDP. The CI job sets
`REQUIRE_DOCKER_E2E=1`, so an unavailable Docker CLI or daemon fails the job
instead of skipping the test.

## Local validation

```powershell
$env:REQUIRE_DOCKER_E2E = '1'
go test -v -count=1 ./tests -run '^TestE2E_DockerComposeFlow$'
```

Result: `PASS` in 9.448 seconds.

The test verified:

- both Control Plane and PDP `/readyz` endpoints before traffic;
- tenant creation followed by authenticated Control Plane policy create and
  publish operations;
- PDP gRPC allow for Alice after publication;
- PDP deny-by-default for Bob before a matching policy; and
- PDP allow for Bob after the second Control Plane publication reaches the
  PostgreSQL-backed policy sync path.

The test creates and removes only its named Compose stack, network and volumes.
Remote CI verification is required before closing the G9 combined gate.
