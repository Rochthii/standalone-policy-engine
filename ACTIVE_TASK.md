# Active Task

ID: V2-CURRENCY-MIG-01 — VERIFY EXISTING ODOO GRANT CURRENCY MIGRATION
Status: COMPLETE — BOUNDED MULTI-COMPANY UPGRADE VERIFIED
Goal: Establish whether a pre-currency Odoo database can upgrade safely with existing delegation grants and preserve a defensible denomination for each legacy maximum.
Scope: Isolated Odoo 17/PostgreSQL test database; simulate the prior grant schema without `currency_id`, retain existing grant data, run the actual module upgrade, and inspect persisted values/constraints. If default migration is unsafe or ambiguous, implement the smallest explicit backfill and verify it. Never use the development `odoo` database or other user database.
Acceptance: Repeatable testbed gate demonstrates pre-upgrade grant rows and exact `max_amount` values survive upgrade; each resulting currency is correct by documented migration rule; rows with unmappable company/currency fail safely or receive an explicitly justified fallback; post-upgrade grant activation and protected-path currency semantics remain valid. Scope/limitations are documented.
Evidence: `deployments/docker/run-odoo-currency-migration.py` initializes a dedicated disposable Odoo 17 database, creates active USD/EUR grants tied to separate delegator companies, removes only the new column to reproduce the prior schema, runs `--update=pdp_authorizer`, and verifies exact maxima/state, company-currency backfill and manifest version advance from 17.0.5.0.0 to 17.0.6.0.0. It then changes a grant to an explicit alternate currency, runs a second module update and verifies the user choice is preserved. Integrated fresh gate passes 78 Odoo post-tests (0 failures/errors), this migration check, mTLS, concurrency/retry/stale-intent, authority ordering and 16 material-edit schedules. `git diff --check`, Python AST and Compose config validation pass.
Residual risk: This is a controlled pre-currency-schema simulation, not a rehearsal on a customer backup or evidence for every historical schema/Odoo upgrade path. Legacy unqualified amounts are interpreted in the delegator's primary company currency; custom external policies remain operator-managed.
Next action: No further work in this follow-up. Git commit/push remain separate and unauthorized in this task.
Plan: `docs/thesis-proposal/THESIS_V2_MASTER_PLAN.md`
Task board: `docs/thesis-proposal/THESIS_V2_TASK_BOARD.md`
Git: Preserve unrelated changes. No stage/commit/push authorized.
