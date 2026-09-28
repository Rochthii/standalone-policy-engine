# Clean research-baseline gate — 2026-09-28

## Result

**PASS — bounded Odoo 17 testbed gate on clean source revision**

- Git revision: `29afd70550645a8ba780345dbb4531fe7638b38a`
- Branch: `main`, aligned with `origin/main` before the run
- Working tree: clean before the run; the gate did not modify tracked source
- Odoo: `17.0-20260908`; disposable database: `odoo_e2e`
- PostgreSQL: pinned `postgres:15-alpine` image in the testbed Compose file
- Result: **78 post-tests, 0 failures, 0 errors**
- The complete wrapper exited with code 0.

This rerun establishes the 78-post-test integrated result on the committed clean revision above. It does not move other studies to this revision: EVAL-01/02/03/04 retain their individually recorded source revisions, dirty-worktree boundaries and raw-artifact hashes.

## Procedure

The repository's Make target was unavailable in this Windows shell (`make` is not installed), so its two Docker commands were run directly and in order:

```powershell
docker compose -f docker-compose.testbed.yml --profile e2e run --build --rm testbed-certgen
$env:PDP_TESTBED_ERP_DATABASE = 'odoo_e2e'
docker compose -f docker-compose.testbed.yml --profile e2e up --build --abort-on-container-exit --exit-code-from testbed-odoo-e2e testbed-odoo-e2e
```

These are the commands in `make test-odoo-e2e`. The runner recreated only its isolated `odoo_e2e` database; it did not target the development `odoo` database. Short-lived test mTLS certificates were regenerated before the gate.

## Observed boundaries

- The mTLS probe rejected a missing client certificate and a wrong hostname; a valid Odoo certificate reached the JWT boundary.
- The isolated currency migration rehearsal passed for legacy USD/EUR grants, preserving exact maximum values and assigning each company's currency. A later module update preserved an explicitly selected alternate currency.
- Two-session direct and approved concurrency/retry checks passed. Fresh-session state checks passed for stale intent and approval invalidation.
- Grant revocation, policy/role/expiry ordering and deferred expiry checks passed at the configured ERP fence boundaries.
- All 16 enumerated material-edit schedules passed with fresh-session commit oracles.

Expected fail-closed errors and serialization retries appear in test logs for negative and concurrency cases; the complete runner exited successfully. The suite also emits a legacy ORM-versus-PDP timing payload with `git_commit: unknown`; that incidental output is **not accepted as benchmark evidence** and is not used in this gate's claims.

## Limits

This is one clean-source run of the pinned local Odoo 17 testbed. It is not production-load, customer-backup, external-effect or cross-system atomicity evidence. It does not replace the earlier EVAL ledgers or upgrade every historical Odoo schema. Reproduction command: `make test-odoo-e2e` from the repository root, with Docker Engine available; the test target recreates `odoo_e2e`.
