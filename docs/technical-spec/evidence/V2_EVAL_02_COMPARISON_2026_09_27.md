# V2-EVAL-02 — Same-workflow A/B/C comparison

> Date: 2026-09-27. Source: `304c1f5` plus the existing dirty worktree and this task's changes; not a release. Scope: one Odoo 17 Community purchase-order confirmation workflow. Comparison: 11 scenarios × 3 variants, not 33 distinct security properties.

## Result and raw evidence

The comparison runner exits 0 with 33 recorded outcomes in [raw JSON](V2_EVAL_02_ABC_20260927T143156Z.json), started 14:31:56 UTC and completed around 14:32:01 UTC. SHA-256: `C87AF8986EBB5E9185CDB706A026D499026E0FF669D0D2808C52D22C25FB2A88`.

The JSON contains before/after fresh-session snapshots, setup responses, final error/return, source-order snapshots for replay, runtime versions, policy revision and per-file source hashes. PASS means the harness's declared positive/negative/persistence oracles passed; it does not mean the intentionally weaker baselines satisfy the proposed invariant.

The passing invocation reused the initialized isolated database after the failed diagnostics below and created new fixture records. It is **not** a fresh-database comparison run. All variants within it used the same database, policy revision, vendor/product, company/currency, creator, delegator and non-superuser purchase-manager execution identity; IDs/nonces/timestamps necessarily differ between case instances.

## Variant contract

| Variant | Retained | Intentionally absent |
|---|---|---|
| A — broad service account | Native Odoo purchase workflow, ACLs and record rules, the same trusted non-superuser service execution context | PDP decision, delegated scope/revocation, exact intent binding, exact-action approval and command consumption |
| B — policy-only | Live mTLS/JWT PDP decision using authoritative ERP amount/currency/creator/department; the same permit/forbid policies as C; ordinary per-PO human approval where required | Human delegation context/grant proof, CBI/witness binding, capability consumption and a final shared authority fence |
| C — proposed | Unmodified normal protected route, except the real CBI flush repair described below; V2 proof, grant checks, exact AC, locked current-state/authority checks, transactional consumption | No test bypass or simulated ALLOW |

The test-only A/B adapters call native confirmation using the existing private sentinel inside a privileged offline Python harness. The addon does not import these adapters and gains no runtime bypass setting or public route. C calls the normal `button_confirm`. This is an **ablation experiment**, not three independently deployed production products.

All cases enter Odoo via a trusted test runner with `env.su == False`, not an HTTP login/tool frontend. B/C reach the actual generated-client/mTLS/JWT PDP boundary. No LLM inference, prompt injection prevention or Odoo frontend authentication is measured.

B's approval is an isolated test SQL row containing PO ID and approver ID. Issuance checks manager role, company, creator/delegator SoD and the live approver policy. It has no material hash, witness, expiry or one-time consumption. For A's review scenarios, the fixture stages the PO at `to approve` and supplies the same ordinary review; this does not imply A requires that approval.

A/B retain the installed addon schema and line guards to hold the business environment constant. Only the trusted experiment adapter ablates delegated authorization. There is no claim that every possible policy-only implementation has these omissions.

## Observed outcomes

“Final” means the observer sees PO `purchase`. “Pending” means `to approve`; “denied” means the target remains `draft`. C negatives also assert no executed command or consumed capability.

| Scenario | A | B | C |
|---|---|---|---|
| Low-value direct allow, 1000 USD | Final | Final | Final; one executed command |
| Approval required, 2500 USD, no approval | Final | Pending | Pending; pending AC |
| Hard policy DENY, same 2500 USD | Final | Denied | Denied; no consumption |
| Independent approval, 2500 USD | Final | Final | Final; one consumed AC/executed command |
| Description edited after review | Final | Final | Pending; old AC invalidated |
| Amount changed 2500 → 2600 after review | Final | Final | Pending; old AC invalidated |
| Grant revoked before execution | Final | Final | Denied |
| Creator is the delegator | Final | Final | Denied |
| Same-record retry | Snapshot unchanged | Snapshot unchanged | Snapshot unchanged; one executed command |
| Same command nonce reused on another PO, same grant | Second PO final | Second PO final | Second PO denied; first remains the source outcome |
| PDP connection unavailable | Final; no RPC needed | Denied, UNAVAILABLE | Denied, UNAVAILABLE |

A plain same-record retry is already idempotent for the observed native workflow fields; do not market that row as a unique C improvement. The cross-record nonce case tests the different command/resource binding property. Snapshots are not exactly-once evidence for arbitrary external effects.

B denies known hard policy violations and refuses execution during an unavailable PDP; it is not a deliberately always-ALLOW mock. The stale-review cases show a distinction between ordinary per-PO approval and exact current-intent approval even while B evaluates current policy at execution.

## Real defect found and repaired

The first committed `approved/C` case became `invalidated: intent_changed` without a business edit. A diagnostic on PO 23 showed:

- Before and immediately after pending creation: line state `draft`, write date `14:30:52.266180`.
- After pending transaction commit: line state `to approve`, write date `14:30:52.433460`.
- Approval issuance made no further line change. The stored and reconstructed CBI differed in line digest and its resulting witness.

The selective line flush in `models/cbi_builder.py` left Odoo's stored related `line.state` recomputation pending. Commit then updated `line.write_date` after the pending CBI had been bound. The repair flushes all pending fields on those order lines before taking the authoritative snapshot. It does not remove the write-version field or loosen equality.

The passing approved/C scenario now commits pending state, issues approval in a second transaction and finalizes in a third. Both post-review edit scenarios still invalidate the old approval. These are executable regressions for the repaired boundary; the earlier EVAL-01 setup combined pending and issuance in a way that did not expose this sequence.

CBI source SHA-256 in the accepted comparison: `02A0656A6E56DDA471C57F542C6EDC56B91436BC9BDF27C003F8A5CE243F005A`. Earlier uncommitted line-lock changes were preserved.

## Failed attempts retained

| Raw result | Failure and disposition |
|---|---|
| [14:25:17](V2_EVAL_02_ABC_20260927T142517Z.json) | Fixture read an ORM field between SQL execute/fetch; ORM reused the cursor. Fetch immediately before further ORM access. |
| [14:26:17](V2_EVAL_02_ABC_20260927T142617Z.json) | B sent amount `1000.00`; the seeded integer evaluator denied it. Use the same exact minor-unit/decimal canonicalization as C. All retained financial fixtures use integral major-unit values; arbitrary fractional-policy support is not claimed. |
| [14:27:23](V2_EVAL_02_ABC_20260927T142723Z.json) | Fixture used UUID hex instead of the CBI command's 32-byte unpadded base64url. Use `secrets.token_urlsafe(32)`. |
| [14:27:51](V2_EVAL_02_ABC_20260927T142751Z.json) | First protected request bootstrapped the ERP policy fence after its transaction snapshot; final lookup failed closed. Perform a real scoped read-only PDP warm-up before comparison transactions. No pending fence is cleared or fabricated. |
| [14:28:57](V2_EVAL_02_ABC_20260927T142857Z.json) | Valid approved/C failed from deferred line-state recomputation. This is a runtime defect, not relabeled as a fixture problem; repaired as above. |

No failed invocation contributes to the 33 accepted outcomes. Expected TLS probe and outage error logs are negative-test observations, not unexplained successful-path errors.

## Reproduction and environment

The isolated Compose project is `pdp-eval-abc`, with its own network, database and TLS/audit volumes. It publishes no database/PDP host ports. No pre-existing Odoo database is deleted by the ABC wrapper; creation fails if `odoo_eval_abc` already exists. `--reuse` explicitly reuses it and creates new fixture POs/grants. The ordinary testbed is not the ABC target.

First initialization (or `make evaluate-odoo-abc`):

```text
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark build testbed-pdp-mtls testbed-odoo-benchmark
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark run --rm testbed-certgen
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark up -d --wait testbed-pdp-mtls
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark run --rm --no-deps testbed-odoo-benchmark
```

Accepted rerun (set `PDP_GIT_COMMIT` to the actual short commit; in PowerShell, `$env:PDP_GIT_COMMIT = git rev-parse --short HEAD`):

```text
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark run --rm --no-deps testbed-odoo-benchmark --reuse
```

Versions: Odoo `17.0-20260908`, Python `3.10.12`, PostgreSQL `15.19`, Linux/WSL2 `6.6.87.2`. Tenant `00000000-0000-0000-0000-000000000017`, revision 3, five active seeded policies. The fifth is an experiment-only hard forbid for department Blocked at the same high amount; setup runs before protected traffic. This seed is not a test of the production policy-publication API.

Images observed for ABC: PDP `sha256:fcf2b20b7f1bfdc6486609d62313aec76627bb0f08c90e9f1eb95f70e3c344ca`; PostgreSQL `sha256:fe0737ba566a2c5b2a28f34433c0a423261900ec17b9bf7ad115e1aae7e57f1b`. Odoo uses the repository's digest-pinned Dockerfile. Raw JSON hashes the mounted Python/client/runner inputs; the dirty commit alone is not a sufficient reproduction fingerprint.

Overlay SHA-256: `9A3B08FE973AA110B0E23A792935435684403E5F043DB3364256C8E3E0070428`. Generator/proposal files are not runtime inputs and were not regenerated.

## Validation

AST parsing passed for the four new Python scripts. Compose configuration validation passed. JSON verification confirms PASS, 33 unique scenario/variant pairs and an exact match to the final CBI file hash. Scoped whitespace validation is recorded at task closure.

Because CBI runtime code changed, the existing fresh-database gate was rerun once on the final runtime inputs:

```text
docker compose -f docker-compose.testbed.yml --profile e2e run --rm --no-deps testbed-odoo-e2e
```

It exits 0 around 14:35:48 UTC. Odoo reports **75 post-tests, 0 failures/errors** at 14:35:00 UTC (102.70 seconds). All independent-session runners pass: baseline/approved concurrency and retry, stale intent, three grant orderings, three committed-authority changes, four after-ALLOW policy/role schedules, two real-clock deferred-expiry rollbacks and 16 material-edit schedules. These units are not added to the 33 comparison rows.

This regression recreated only the existing disposable `odoo_e2e` database through its standard runner; the normal `odoo` database was not recreated. The main test PDP image was `sha256:17ea5516c21078402c23b19f08ebc46511259f04d8fa92595750c21f2d3a9880`; no Go source changed in this task. No unchanged Go suites or DOCX/PDF generation were rerun.

## Interpretation and limits

- This is a selected 11-scenario correctness comparison, not a complete rerun of every EVAL-01 matrix ID through each baseline.
- A/B/C bundle different controls. Delegator/revocation outcomes reflect the chosen identity model as well as binding; they do not isolate the marginal effect of the hash or prove a general weakness in all policy engines.
- No arbitrary-field discovery, arbitrary extensions/SQL, compromised trusted PEP, multi-tenant scale, concurrent A/B/C schedules or external-effect guarantee was evaluated here.
- The current configured-writer, intact-trigger, trusted-clock and deferred-validation-time limits still apply. Warm-up excludes cold-start fence behavior from the comparison, which is reported separately above.
- This task establishes no latency distribution or speedup. The broader regression wrapper happens to contain a historical benchmark; its incidental output is not the boundary-separated V2-EVAL-03 study.
- Odoo/PDP authentication transport probes pass, but frontend identity handoff and a real LLM tool stack are outside this experiment.
- No production-readiness, SAP compatibility, legal compliance or general ERP-security claim follows.

Next: V2-EVAL-03 measures separate evaluator, proof/capability, gRPC/mTLS, locked state and ERP mutation boundaries using explicit inclusion/exclusion rules and retained raw samples.
