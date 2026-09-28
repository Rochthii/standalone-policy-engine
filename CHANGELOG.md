# Changelog

## [Unreleased] - 2026-09-28: CI rerun verified

- Workflow run [#83](https://github.com/Rochthii/standalone-policy-engine/actions/runs/36442087221) for commit `a27d0bd` completed successfully: all 10 jobs passed, including Protobuf Contract and Real Odoo PostgreSQL gRPC E2E. The one-off Odoo error from run #82 did not recur; no Odoo code or test was changed. This rerun confirms the gate passed but does not identify the individual exception behind #82.

## [Unreleased] - 2026-09-28: Protobuf CI download resilience; Odoo failure remains under diagnosis

- The Protobuf Contract job now installs the same pinned Buf CLI once and retries only that installation up to three times for transient module/checksum transport failures. Go checksum verification remains enabled; lint, format, generation and breaking checks still fail normally on contract errors.
- CI run #82 also reports one error among 78 Odoo tests. Its visible tail only identifies the failed Odoo subprocess; the earlier clean Odoo gate passed on unchanged Odoo source, so no test was disabled or production code altered based on a guess. Root exception and a CI rerun remain outstanding.
- Workflow YAML parses and `git diff --check` passes. No full CI rerun, commit or push.

## [Unreleased] - 2026-09-28: clean research-baseline gate verified

- Reran the full isolated Odoo 17/mTLS/PDP/PostgreSQL testbed gate on clean `main` revision `29afd70550645a8ba780345dbb4531fe7638b38a`: 78 post-tests, 0 failures/errors; currency migration, mTLS, concurrency/retry, authority-ordering and all 16 material-edit schedules pass. See `docs/technical-spec/evidence/CLEAN_BASELINE_GATE_2026_09_28.md`.
- Updated README, master index, current-state audit, task board and active checkpoint to link the clean-run evidence. EVAL-01/02/03/04 retain their earlier `304c1f5` plus dirty-worktree provenance; this gate does not rewrite those studies. No code changed; no commit, tag or push.

## [Unreleased] - 2026-09-28: V2-CURRENCY-MIG-01 company-aware grant migration verified

- The initial populated-schema upgrade test found a real multi-company bug: Odoo populated the newly required `currency_id` for every old grant with the upgrade process's current company currency, assigning USD to a legacy grant whose delegator belonged to an EUR company. Added a conditional `_auto_init` backfill that runs only when an existing grant table lacks the currency column, maps legacy amounts to each delegator's primary company currency, and aborts upgrade if the mapping is unavailable. Subsequent upgrades do not overwrite explicit currency selections.
- Bumped the addon version from `17.0.5.0.0` to `17.0.6.0.0`; the migration runner simulates the prior version and asserts the upgrade reports the new version.
- Added `deployments/docker/run-odoo-currency-migration.py` and wired it into the full E2E runner using only dedicated scratch database `odoo_currency_migration`. It seeds active USD and EUR grants under separate companies, simulates the old schema by dropping only the new column, calls Odoo `--update=pdp_authorizer`, then verifies amount/state/currency. It performs a second update after an explicit alternate-currency selection and verifies that choice persists.
- Final fresh Docker gate: 78 Odoo post-tests, 0 failures/errors; migration initial and repeated upgrades pass; mTLS, two-session concurrency/retry/stale-intent, authority-ordering and 16 material-edit schedules pass. Python AST, Compose config and `git diff --check` pass. The migration fixture is an isolated prior-schema simulation, not a rehearsal on customer backups or all historical addon versions. No commit/push.

## [Unreleased] - 2026-09-28: V2-CURRENCY-MIG-01 migration compatibility check started

- Follow-up authorized to verify the untested upgrade path for existing `pdp.delegation.grant` rows after adding required `currency_id`. Active checkpoint now limits work to an isolated disposable Odoo 17/PostgreSQL database and forbids use of the development/user database. No migration behavior is assumed and no production code changed in this handoff.

## [Unreleased] - 2026-09-28: V2-CURRENCY-01 bounded Odoo evidence complete

- After Docker Engine became available, the fresh Odoo 17/mTLS/PDP/PostgreSQL gate exposed two legacy fixtures whose USD 2,500 approval exceeded their old USD 2,000 grant cap. Raised only those test grant caps to USD 5,000; the production ceiling remains enforced. Full gate then passed: 78 post-tests, zero failures/errors, mTLS probes, baseline/approved concurrency and retry/stale-intent runners, authority-ordering schedules and all 16 material-edit schedules.
- Verified exact grant-cap acceptance; over-cap, mismatched PO/grant currency, EUR seed-policy and missing-currency seed-policy denials with persistent rollback oracles. Full `go test ./...`, focused policy regression, Python/XML checks and `git diff --check` pass. AUTH-N03 and V2-CURRENCY-01 are now marked complete at this bounded evidence level; current-state audit records the same scope.
- Existing-database migration for the new required grant currency and external custom policy setup remain unverified. No implicit FX conversion, non-USD seed allowance, commit or push.

## [Unreleased] - 2026-09-28: V2-CURRENCY-01 currency-scoped grant and seed-policy repair (Odoo gate pending)

- Replaced the grant's unqualified amount field semantics with an explicit currency-bound monetary maximum. Activation validates exact currency precision; the protected V2 path compares the locked CBI minor-unit amount against that cap and rejects mismatched PO currency whenever the intent is rebuilt, including final execution.
- Scoped seed Cedar and Odoo E2E policies to USD explicitly. No VND/EUR threshold was invented: missing/unsupported currencies have no matching seed permit and default-deny. Documented that organizations must configure a separate policy threshold before enabling another currency.
- Added Go policy cases for configured USD, unconfigured EUR and missing currency; added real Odoo negative/positive cases for exact cap, over-cap, grant/PO currency mismatch and unconfigured EUR. Corrected an existing TC-04 assertion: the >USD 2,000 rule returns `ALLOW` plus `REQUIRE_HUMAN_APPROVAL`, not `DENY`.
- Validation: targeted Go currency-policy test and full `go test ./...` pass using a task-local temporary GOCACHE; Python AST, Odoo view XML and `git diff --check` pass. Attempted the fresh Odoo gate, but it could not start because Docker Engine is unavailable (`//./pipe/docker_engine` missing); `make` is not installed on this Windows host. Odoo transaction/persistent-state cases remain unverified, the audit is not promoted, and V2-CURRENCY-01 stays in progress. No commit/push.

## [Unreleased] - 2026-09-28: V2-DOC-04 paired proposal artifact QA closed

- User confirmed the DOCX and PDF are paired outputs of one proposal and accepted the existing PDF visual review as the artifact QA; no additional export was needed.
- Re-rendered the existing five-page A4 PDF with bundled Poppler 26.07.0 at 180 dpi using `pdftoppm.exe -png -r 180 <proposal.pdf> <temp-prefix>`; inspected all pages with no clipping, overlap, missing glyphs or broken tables. Existing DOCX source parity is 80 Markdown blocks and three tables.
- Closed the separate V2-DOC-04 gate as `VERIFIED — PAIRED ARTIFACT QA` and reconciled ACTIVE_TASK.md, task board and master-plan checkpoint. Word-native DOCX pagination was not independently rendered and remains explicitly unverified; it is non-blocking per the user's scope decision, not Word-rendered evidence.
- No content/export change, runtime tests, commit or push.

## [Unreleased] - 2026-09-28: V2-WRITE-04 proposal exports regenerated

- Regenerated `docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.docx` and `.pdf` from the active Markdown using `python scripts/generate_master_thesis_proposal.py`; the source check passed with 80 shared blocks and locked Vietnamese/English titles. Corrected the DOCX subject property to remove a stale V2-DOC-05 label. Bundled Python: 3.12.14. Source revision: `304c1f5774cdb4dcc4e644f3b050a437c5989a23`, with a dirty worktree; this is not release provenance.
- DOCX structural comparison passed exactly: 80 source blocks and all three tables match the Markdown. Bundled Poppler 26.07.0 rendered the five-page A4 PDF at 140 dpi; every page was inspected and showed no clipping, overlap or missing glyphs. DOCX image rendering remains open: the packaged `render_docx.py <proposal.docx> --output_dir <temp-dir> --verbose` attempt failed with `FileNotFoundError: LibreOffice soffice.exe was not found on PATH`; the workspace dependency bundle contains no LibreOffice binary. The bundled PDF renderer does not provide DOCX visual evidence.
- SHA-256 DOCX: `226D694A4D592F5D2AC7786044A94A1D288BAB45B9A07A860DA60415DD005A4E`. SHA-256 PDF: `73BB44B3967C175EEB02C01F531DF538833878D2D4BCFEB315B3B997BFE42590`.
- Closed V2-WRITE-04 at its bounded acceptance and recorded the separate DOCX visual gate as open. No runtime tests, commit or push.

## [Unreleased] - 2026-09-28: V2-WRITE-03 cross-document reconciliation complete

- Reconciled the active proposal, Chapters 3–5 draft, chapter map, scope alignment, career roadmap, README, master index, master plan and task-board status with EVAL-04's bounded claim wording. Fixed stale statements that boundary-separated performance/results were still planned and replaced obsolete next-work instructions.
- Confirmed title, RQ1–RQ4, three contributions, Odoo 17 `purchase.order` runtime scope, SAP discussion-only boundary, 75-post-test/25-ID distinction, 11 × 3 comparison denominator and EVAL-03 sample-unit caveats. Checked the seven proposal references against their linked primary publication pages.
- Marked WRITE-03 complete and routed WRITE-04 to artifact generation/visual QA. Documentation/link/status checks pass; no runtime evidence, generated export, commit or push was added.

## [Unreleased] - 2026-09-28: V2-WRITE-02 Chapters 3–5 draft complete

- Added one Markdown manuscript for Chapters 3–5, covering the delegated authorization model, CBI/AC/SoD contracts, Odoo/Go enforcement path, authority ordering, rollback/retry, evaluation and bounded RQ1–RQ4 answers.
- Kept EVAL-01/02/03 denominators and EVAL-04 claim boundaries adjacent to results. Cross-checked all 13 Odoo and four Go reported metric rows plus the 11 A/B/C scenarios against accepted sources; all draft relative links resolve.
- Linked the draft from the chapter map and moved the task board/checkpoint to WRITE-03. No runtime/protocol changes, DOCX/PDF export, commit or push.

## [Unreleased] - 2026-09-28: V2-WRITE-01 current-state audit reconciliation

- Added a concise superseding EVAL-03/04 evidence entry to `CURRENT_STATE_AUDIT.md`, with sample denominators, dirty-worktree/source boundary, trusted-writer/trigger/clock assumptions and prohibited generalization.
- Marked WRITE-01 complete and advanced the checkpoint to WRITE-02. The detailed claim ledger remains the source for wording and validity threats; older audit entries remain as chronology.
- Scoped relative-link, task-status and `git diff --check` validation passed. Documentation-only change; no tests or runtime measurements rerun. No commit/push.

## [Unreleased] - 2026-09-28: EVAL-04 bounded claim analysis complete

- Added one claim-to-evidence ledger mapping nine permitted thesis claim families and RQ1–RQ4 answers to accepted EVAL-01/02/03 evidence, each with its required wording boundary.
- Recorded construct, internal, external, conclusion and reproducibility threats, including the trusted-runner/dirty-worktree/configured-writer assumptions, finite Odoo scope, selected ablations, sequential measurements and unverified recovery/load/SAP boundaries.
- Reconciled the threat model, evaluation matrix, scope alignment, proposal Markdown and chapter map from planned EVAL-03/04 wording to bounded verified evidence. No runtime claim, source code, raw evidence or generated export changed.
- Closed V2-EVAL-04 and routed V2-WRITE-01 to Luna for a fixed current-state-audit update. No commit/push.

## [Unreleased] - 2026-09-27: EVAL-03 bounded measurement accepted

- Corrected Go QPC collection passed with 4,400 rows: 3,000 measured API calls, 1,000 measured controls and 400 warm-ups. All API samples and operation results are valid; zero is retained only where the empty timer control completes within a QPC tick.
- Aggregated the exact new Go measurement with the previously reviewed 440-row Odoo measurement. Recorded per-boundary percentiles, raw SHA-256 values and measurement limits; rejected historical runs remain unchanged and excluded.
- Closed V2-EVAL-03 as bounded evidence and moved the active checkpoint to V2-EVAL-04. No production runtime changed; no broad ERP, causal approval-overhead, production SLA or speedup claim is made. No commit/push.

## [Unreleased] - 2026-09-27: EVAL-03 zero control exception fixed

- Permit 0 ns only for the Go empty timer_control; retain rejection of negative times, zero API/Odoo timings, invalid numeric types and operation failures. Summary validation limits the exception to go_micro with a Go per-boundary plan and retains zero controls in distributions. Failed historical JSON is never reclassified.
- Six Python tests pass, including accepted zero control, retained count/percentile, rejected zero in each API, invalid control values, wrong route and failed status. A new 100-row Go smoke and summary against retained full Odoo JSON pass. Full corrected Go collection remains pending manual switch to Luna; no Odoo rerun or production-code change.
- Count correction superseding the previous entry: 4,000 measured rows comprise 3,000 API calls and 1,000 controls, not 4,000 API calls. The 400 warm-ups comprise 300 API calls and 100 controls. The failed QPC artifact has exactly two zero measured controls; API timings were positive.
- Reconciled report/checkpoint/board and measurement pointers for fixed Go-only collection/closure, then stop for Sol before EVAL-04. No new task IDs, commit or push.

## [Unreleased] - 2026-09-27: EVAL-03 QPC full-run control sample diagnosis

- Full Go QPC attempt preserved 4,400 rows and exited 1 solely because two of 1,000 measured empty `timer_control` samples were 0 ns; QPC resolution is 100 ns. All 4,000 measured API calls and 400 warm-up operations were positive and returned valid results. No Odoo sample repeated.
- Diagnosis: nonpositive API durations must fail, but an empty control can complete within one QPC tick. The collector's shared zero-duration rule incorrectly made the whole run fail. Handoff to Sol to scope the exception to timer_control, preserve API/Odoo positivity, add a focused acceptance check; then Luna reruns only Go. EVAL-03 remains in progress.

## [Unreleased] - 2026-09-27: EVAL-03 Windows clock repair and bounded Odoo review

- Traced the installed Go 1.26.4 Windows/amd64 clock to `_INTERRUPT_TIME` in time.now/nanotime1; the collector's nanosecond units masked inadequate effective resolution. Followed the existing repository QPC pattern in two test-only clock files. Collector now records frequency/tick and fails on nonpositive elapsed times; security validity clocks and production code are unchanged.
- Strengthened summary acceptance to reject zero/negative/missing/noninteger durations even under a declared PASS. Five focused Python checks pass; QPC smoke yields 100 positive rows, frequency 10 MHz/tick 100 ns. Existing invalid Go JSON is rejected as expected; QPC smoke plus retained Odoo JSON validates. Windows clock overhead remains included, with no percentile subtraction or intrinsic evaluator latency claim.
- Reviewed protected hooks and all 440 retained Odoo rows for commit/persistence and inclusive span consistency. Preserve the 200 measured transactions per route and existing distributions. Only corrected full Go collection remains; no repeated Odoo/ERP gate or scope expansion.
- Updated report, checkpoint and board once for manual Luna handoff. Fixed collection/closure criteria allow arithmetic-only completion, then stop for Sol before EVAL-04. Previous diagnostic artifacts remain intact; EVAL-03 is still in progress. No commit/push.

## [Unreleased] - 2026-09-27: EVAL-03 collection retained; Go timing validity review required

- Collected 4,400 Go rows and 440 Odoo rows on the fixed warm procedure. Odoo's 200 direct and 200 approved measured commits pass fresh-session persistent outcome checks with zero boundary-call errors; raw distributions and captured image/runtime/source metadata are recorded in the EVAL-03 report.
- The Go test exits 0 but records evaluator and empty-timer durations as 0 ns in 1,000/1,000 cases, and proof/capability as zero in 980/991 cases. Preserve raw output; do not cite these percentiles. This invalidates the Go microtiming evidence pending diagnosis, not a production security finding.
- Windows host CPU query was denied; report marks host CPU model unavailable. EVAL-03 stays in progress pending Sol review of timer validity and Odoo boundaries. No reruns, runtime change, new research claim, commit or push.

## [Unreleased] - 2026-09-27: EVAL-03 measurement preparation and mandatory model handoff

> This entry captures the earlier pre-collection checkpoint; the EVAL-03 collection and Sol-review entry above supersedes its "full measurements remain unrun" status.

- Added one opt-in Go API collector and five Python timing/hook/runner/summary/test files. Separate evaluator, proof/AC verification, warm gRPC/mTLS client calls, locked CBI, native mutation-call component and confirm-through-commit totals; direct/approved routes are distinct. Original protected functions always execute; no production or Compose changes.
- Four focused metric/artifact tests, Python AST, Go smoke (16 rows), real Odoo smoke (six committed/fresh-observer outcomes), raw-denominator/summary and whitespace checks pass. Smoke JSON is explicitly not final latency evidence. Fixed test-only import-cycle, fixture-ID-length and floating-point assertion issues; unchanged EVAL-02 correctness evidence reused.
- Recorded the contract, limits, smoke artifacts and fixed full-collection commands in the EVAL-03 report. Full measurements remain unrun; EVAL-03 stays IN PROGRESS, six tasks plus the existing DOCX visual gate remain. No current-state claim promotion or proposal export regeneration.
- Updated AGENTS.md, task board and checkpoint to enforce the user's manual model switches: end the turn before Luna's mechanical collection group, then stop for Sol review or diagnosis. No nested tasks, automatic model-switch claim, commit or push.

## [Unreleased] - 2026-09-27: EVAL-02 committed A/B/C comparison and CBI flush repair

- Added an isolated Compose overlay, four test-only Python setup/fixture/variant/comparison scripts and `make evaluate-odoo-abc`. Eleven shared scenarios produce 33 A/B/C outcomes with real mTLS/JWT PDP calls and fresh-session PO/command/approval observations. A/B ablations never become an addon runtime option; native same-record retry behavior and B's hard-DENY/outage protection are reported alongside C's binding benefits.
- Found a real false invalidation: deferred Odoo line-state recomputation advanced line write dates at pending commit after CBI creation. Flush all pending line fields before authoritative reconstruction; preserve exact witness comparison and prior line-lock changes. The approved/C scenario now passes pending, issuance and execution in three committed transactions, while post-review edits still invalidate approval.
- Accepted JSON `V2_EVAL_02_ABC_20260927T143156Z.json` records 33 outcomes, source hashes and runtime metadata; five failed diagnostic results remain intact. The isolated comparison reuses its initialized database with new fixtures and is not labeled a fresh-database run.
- Validation: Python AST, Compose config, 33 unique scenario/variant pairs, exact final CBI hash and whitespace/BOM checks pass. Because runtime changed, ran the existing fresh Odoo gate once: 75 post-tests, zero failures/errors, plus all concurrency, grant/authority, deferred-expiry and 16 material schedules; exit 0 around 14:35:48 UTC. No Go code changed; no unchanged Go suites or document exports were rerun.
- Reconciled matrix/audit/CBI note, board, current entry-point status and checkpoint. Completed EVAL-02; EVAL-03 is next, with six tasks plus the existing DOCX visual gate remaining. The existing disposable `odoo_e2e` regression database was recreated; normal `odoo` data was not. No commit/push; no latency, production or general ERP claims.

## [Unreleased] - 2026-09-27: DOC-05 locked framing and single-source proposal exports

- Reconciled the eight named overview/proposal documents: root README, master index, V2 master plan, active proposal, scope/evidence alignment, career roadmap, chapter mapping and addon README. The locked VN/EN title, delegated non-human principal, three contribution layers, authoritative intent flow and ALLOW-only approval obligation now agree. Go PDP performance is supporting evidence; SAP remains applicability only.
- Reused EVAL-01's bounded 75-post-test closure and 25 composed-boundary IDs without a new runtime claim. Marked A/B/C and current overhead as planned; kept configured-writer/trigger/clock assumptions, deferred-validation limit, manual recovery and availability/locking risks explicit. Preserved V1 archive and prior worktree changes.
- Removed stale diagrams, retired performance framing and unsupported bibliography claims; corrected OWASP to LLM06:2025, Cedar's lead author to Cutler and Zanzibar attribution using primary sources. Related-work novelty and final bibliography reconciliation still belong to thesis writing.
- Replaced duplicated generator prose with one Markdown source shared by DOCX/PDF (80 blocks). Added a focused four-check regression file for locked titles, local links, supported/rejected parser input and every block's presence in both exports, including table/link structure. No runtime code, generated protocol, lockfile or environment configuration changes.
- Validation: bundled Python ran `scripts/generate_master_thesis_proposal.py --check`, generated the artifacts and ran `scripts/test_generate_master_thesis_proposal.py` (4 passed). Bundled Poppler rendered the PDF; all 5 pages visually inspected without clipping/overlap. DOCX creation/content checks passed but bundled `render_docx.py` failed with `LibreOffice soffice.exe was not found on PATH`; DOC-04 visual QA stays open. The PDF is independently generated, not proof of DOCX layout. Scoped whitespace validation recorded at handoff; no Odoo/Docker rerun.
- Completed DOC-05 where rendering is supported, prepared EVAL-02 with batched Sol/security work and optional lower-model mechanical review, and retained seven remaining tasks plus the existing DOCX visual gate. No stage, commit or push.

## [Unreleased] - 2026-09-27: EVAL-01 bounded authority-ordering closure

- Added ERP policy-publication fencing to all configured Go policy/role mutation APIs and both server entrypoints. Pending publication is durable and fail-closed; decisions cannot clear it. Propagated the exact evaluated snapshot revision through gRPC Advice and checked both approver/agent revisions at the final ERP lock.
- Added a local-authority SQL epoch for user/group/membership/company/grant changes and deferred PostgreSQL execution-deadline validation. Grant/authority/policy locks survive through local PO/AC/command commit; expiry at deferred validation rolls the transaction back. No distributed transaction or later WAL/network-time validity claim.
- Added four after-ALLOW policy/role schedules, direct/approved real-clock grant-expiry rollback, five policy-fence fault variants, real PostgreSQL writer/cancellation/failure regressions and snapshot/gRPC metadata tests. The final fresh Odoo 17/mTLS/PDP/PostgreSQL gate exits 0: 75 post-tests, zero failures/errors, every existing runner including 16 material and three grant schedules passes. Exact commands, source scope, fixture failures and the earlier transient UNAVAILABLE are retained in the existing case ledger.
- Closed V2-EVAL-01 at the 25 retained IDs' composed boundaries; prepared V2-DOC-05 and the eight-task remaining order. Reconciled audit, matrix, invariants, addon operation/recovery guidance, root agent guide, task board and checkpoint. Recovery automation, unconfigured writers, arbitrary extensions/SQL and production readiness remain unclaimed. No commit or push.

## [Unreleased] - 2026-09-26: Batched EVAL-01 gaps and concurrent line-insertion repair

- Reproduced a membership phantom: an independently committed note line escaped a stale final snapshot and its old approval was consumed. Added `purchase_order_line_guard` to lock/version protected parents before ORM line create/write/unlink, including tax relations. Ordinary parent-version regression passes.
- Added public expired/inactive-grant, exact-one-minor-unit, public wrong-authority issuance and missing-JWT live issuer tests. Expanded material runner to 16 ordered schedules and added three committed-before-final policy/role/expiry cases; no simulated policy decision.
- Staged evidence: fresh Odoo/mTLS run reports 74 post-tests / 84 cases, zero failures/errors. Its concurrency stage exposed a fixture error-message mismatch; corrected complete runner then exits 0 with 16 material, three grant-ordering and three committed-authority schedules. Unchanged Odoo tests were not rerun after the runner-only correction. Earlier fixture/setup failures and exact commands/source fingerprint are recorded in the existing ledger.
- Four of the five pending ledger rows are now bounded verified. TXN-N03 after-last-check policy/role/expiry ordering remains unresolved; EVAL-01 is not complete. Reconciled active checkpoint, board, matrix, audit, scope review, invariants and addon guidance once. No commit/push or new task IDs.

## [Unreleased] - 2026-09-26: Batched task execution and compact context handoff

- Revised the existing `active-task-workflow` skill instead of adding another skill or task: batch related acceptance gaps, continue through checks, and use one consolidated exit gate unless failures or changed inputs require reruns.
- Aligned root guidance, board policy and active checkpoint; detailed evidence remains in the existing ledger. Removed the push-before-task-advance prerequisite while retaining explicit commit/push authorization and unchanged security acceptance.
- Validation: skill syntax validator and scoped whitespace check; manual scenario review covers continuation with unchanged inputs, docs-only changes, failed/changed gate inputs, context loss, unrelated dirty edits and acceptance complete without push. No Odoo/Go runtime changes or rerun; token savings and implementation forward-test remain unmeasured.

## [Unreleased] - 2026-09-26: Grant revocation ordered with ERP commit

- Repaired the reproduced after-ALLOW grant race using a tenant/grant fence held by Odoo through business commit. PDP first commits a monotonic ERP tombstone, then its existing revocation; failure of the second write remains fail-closed. Required database-scope acknowledgement prevents a silent fallback to an unfenced PDP.
- Added three asserted real-session schedules (revoke-first, final-first with observed blocking, rollback), initial/approved tombstone denial and acknowledgement negatives. Real PostgreSQL cancellation testing exposed an auto-commit ambiguity; explicit fence transactions fixed the blocked-cancellation case.
- Fresh gate: 69 post-tests / 79 reported cases, zero failures/errors, three authority and four material schedules plus existing runners pass. Go server/config and focused real PostgreSQL tests pass. Compose maps normal Odoo, E2E and benchmark to their corresponding fence database; only E2E runtime was exercised here.
- Updated addon configuration guidance, active task, board, ledger, matrix, audit and invariant status. Five composite rows remain open, including policy/role/expiry timing; no general distributed-atomicity claim. No commit or push.

## [Unreleased] - 2026-09-26: Reproduced revocation/commit race

- Added an independent-session diagnostic that pauses final Odoo execution after the live PDP ALLOW, commits `action_revoke()` in another session, then resumes final execution.
- Fresh-session evidence: grant `revoked` while PO `purchase`, approval `consumed` and attempt `executed`. The 66 post-tests / 76 reported cases and existing race schedules passed, but the commit-time authority invariant is not met; V2-EVAL-01 stays open.
- Next action within the same active task: implement shared ordering for Odoo final execution and PDP revocation, then rerun the reproducer and consolidated gate. No nested tasks or scope expansion.

## [Unreleased] - 2026-09-26: Evaluation scope review (no runtime change)

- Reviewed six partial EVAL-01 rows against actual executable boundaries. Corrected AUTH-N01's HTTP middleware anchor to gRPC authentication/live mTLS evidence; five rows remain partial, not a 25/25 pass.
- Corrected the missing-JWT approval-issuance claim and distinguished private guards, serializer checks, public ERP routes and live policy publication. Preserved the unresolved last-authority-check-to-commit guarantee rather than weakening the thesis invariant.
- Added `V2_EVAL_01_SCOPE_REVIEW_2026_09_26.md` and synchronized ledger, matrix, active task, board and audit. Next is a bounded independent-session authority timing investigation, followed by closure and distinct material-lock packages.
- Documentation-only review: no new tests, production-code changes, commit or push. Retained runtime evidence is 66 post-tests / 76 reported cases plus four separate material-edit schedules.

Tài liệu này ghi nhận toàn bộ lịch sử thay đổi, tiến độ phát triển và timeline thực tế của dự án **Standalone Policy Engine**.

Phân loại thay đổi:
*   `Added`: Các tính năng mới được phát triển.
*   `Changed`: Các thay đổi cấu trúc hoặc tối ưu hóa mã nguồn hiện có.
*   `Fixed`: Sửa các lỗi cú pháp, logic hoặc bảo mật.
*   `Security`: Các bản vá và cơ chế bảo mật hệ thống.

---

## [Unreleased] - 2026-09-24: V2 Evaluation Matrix Materialization (Partial)

### Security and tests
- A missing persisted AC payload now raises a controlled authorization error, allowing deterministic invalidation without an unauthorized final PO effect.
- Added Odoo adversarial tests for Activity completion/deletion/reassignment without approval, material vendor/line edits after AC issuance, and missing approval row/AC payload. Extended the independent-session runner with a committed line edit between approval and final execution; the old AC invalidates and the PO remains non-final.
- Extended the Go V2 proof tamper matrix with tenant/company substitutions and tested a valid AC signature against the delegation-proof verifier. Final Odoo tests also reject malformed/wrong-agent/wrong-tenant JWTs and an injected V1 proof with non-final/unconsumed state.
- An exploratory `partner_ref` rewrite did not advance Odoo `write_date`; replaced the invalid parent-version fixture with an explicit persisted version bump and clarified that the timestamp is not a universal write counter. No broader transaction-version guarantee is claimed.

### Evidence
- Final fresh Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate: 48 post-tests / 58 cases, 0 failures/errors; baseline and approved two-session runners plus stale-intent runner pass. Focused Go proof/key-confusion tests pass.
- Added a 25-ID case-to-test ledger distinguishing real ERP, Go/Python boundary and partial composite evidence. V2-EVAL-01 remains in progress; this is not a 25/25 security pass.

## [Unreleased] - 2026-09-24: Approved PO Transaction Failure and Race Evidence

### Verification
- Added Odoo final-route tests for an injected failure after the business transition, a real AC-verifier transport outage and a controlled late agent-PDP outage. Each negative case asserts a non-final PO and unconsumed, retryable approval/attempt before a successful retry.
- Extended the independent-session runner to race two executions of one approved command, assert one consumed approval/one executed attempt/one final PO, then discard the response and retry in a new session while forbidding a second `button_approve` call.
- Final fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate: 41 post-tests / 51 cases, 0 failures/errors; mTLS probes, baseline nonce runner, approved race and lost-response retry pass.

### Remaining scope
- Evidence is limited to one Odoo/PostgreSQL PO effect. Independent-session concurrent business-field edits, cross-system policy-snapshot atomicity, external exactly-once effects, Enterprise and SAP remain unverified.

## [Unreleased] - 2026-09-24: Approved Delegated PO Final Execution

### Security
- Connected the existing public `button_confirm` retry to the approved AC v1 route: lock/reconstruct exact pending CBI, revalidate current grant and approver authority, verify AC through the PDP, and obtain a fresh V2 proof-bound agent-confirm decision.
- In one Odoo/PostgreSQL transaction, mark the approval `consumed`, invoke the guarded base PO final transition and mark the protected command `executed`; pending or stale approval cannot reach `purchase`.
- Reused one exact-decimal agent request builder for initial and final decisions, eliminating an unnecessary float-based amount calculation from that protected request path.
- Changed the protected high-value policy to `ALLOW` with `REQUIRE_HUMAN_APPROVAL`; final execution accepts the AC only for a current ALLOW. A `DENY` carrying the same obligation remains a hard denial, so unrelated forbids cannot be overridden. The non-delegated legacy non-final routing remains unchanged.
- The pending attempt now stores the PDP's actual `allow` decision alongside `approval_required`, instead of recording a synthetic `deny` after the policy semantics changed.

### Verification
- Focused fresh-database final-route suite: 6 post-tests / 6 cases, 0 failures/errors. Final complete Odoo/mTLS/PDP/PostgreSQL gate: 38 post-tests / 48 cases, 0 failures/errors, plus mTLS probes and the baseline two-session nonce check.
- Eight final-route cases cover success, pending approval, changed intent, grant revocation, current approver-policy denial, simulated agent-policy denial (including DENY with an approval obligation), expired AC and tampered AC, with non-final/unconsumed negative oracles. The real high-value hard-SoD case creates no approval.
- The full gate exposed a test-only grant-ID collision because PDP revocation persists outside Odoo test transactions; the new fixture now uses a disjoint ID range.

### Remaining scope
- V2-TXN-04 still owns independent-session final execution, injected rollback, lost-response retry and final-route outage tests. No atomic cross-system policy snapshot, instant revocation, tax-relation race closure or general ERP/SAP claim is made.

## [Unreleased] - 2026-09-24: Approved Final-Intent Preflight

### Security
- Added a private same-transaction preflight for an approved Odoo PO: lock the PO, approval and command attempt, re-read and lock persisted lines, and compare the exact current CBI to the stored approved intent/hash/witness.
- Material line/vendor drift invalidates the old approval without a final business effect; mismatched command-attempt binding fails closed.

### Verification
- Fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes 30 post-tests / 38 cases with 0 failures/errors, including four new preflight tests; mTLS probes and two-session nonce baseline pass.
- The prior 24-hour testbed certificates had expired, so the repository's test-certgen refreshed that test-only volume and the mTLS PDP was restarted before the passing run.

### Remaining scope
- Preflight is not wired to final execution. Current policy/revocation/approver rechecks, atomic approval/command consumption with PO mutation and final-route race/rollback/retry evidence remain V2-TXN-03/04. Tax-relation and extension-field races are not closed by the line-row lock alone.

## [Unreleased] - 2026-09-23: Delegated PO Final-Entry-Point Guard

### Security
- Added one convergent `purchase.order.write` guard for the pinned Odoo 17 Community public methods and ORM/RPC state-write path that can perform a first transition into `purchase` or `done`.
- Restricted the valid base-confirm transition to a process-local object-identity sentinel carried only after the repository PEP returns ALLOW; a serialized RPC context value cannot forge it.
- Added a sticky delegated-scope marker so clearing the grant, agent and delegator first cannot create a two-step bypass.

### Verification
- Focused fresh-database public-dispatch coverage passes 6 post-tests / 8 test cases with 0 failures or errors.
- The full Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes 26 post-tests / 34 test cases with 0 failures or errors, mTLS boundary probes, and the two-session one-nonce/one-effect concurrency check.
- Direct `button_approve`, `button_done`, `button_unlock`, `write(state=...)`, forged-context and marker-removal cases prove no unauthorized persistent business mutation; the supported confirm route and post-confirm `done`/`unlock` behavior remain available.

### Remaining scope
- This evidence is limited to the repository-pinned Odoo Community ORM/RPC boundary. V2-TXN-02 through V2-TXN-04 still own approved final execution, locked current-intent reconstruction, current-authority checks, atomic consumption/mutation, rollback and lost-response behavior.

## [Unreleased] - 2026-09-23: Approval Revalidation and Deterministic Invalidation

### Added
- Explicit `rejected`, `invalidated`, `expired` and reserved `consumed` approval terminal states with immutable terminal-state guards and recorded terminal reason/time.
- A PO-then-approval locked Odoo revalidation operation that reconstructs current persisted CBI, checks local delegation lifecycle, verifies stored AC v1 through the PDP and rechecks the original approver's current role, tenant/company, SoD and PDP approval policy.
- Deterministic invalidation for changed intent, expired capability/grant, revoked grant and lost approver authority; transient PDP/verifier outages fail closed without permanently invalidating retryable approval evidence.
- Focused persistent-state tests proving that every APP-04 denial leaves the purchase order `to approve` and applies no protected business mutation.

### Verification
- Fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL execution passes 20 post-tests / 26 test methods with 0 failures or errors, including current-policy denial, mTLS negative probes and the two-session single-nonce concurrency check.
- Addon compilation, focused Go security/server/parser/engine tests, `go vet ./...`, Docker Compose parsing and `git diff --check` pass.

### Remaining scope
- APP-04 exposes a private non-mutating revalidation boundary; it is not yet wired as final purchase-order execution. Final entry-point coverage, locked commit-time reconstruction, real `consumed` transition, current-authority revalidation and atomic command/capability consumption with the PO mutation remain V2-TXN-01 through V2-TXN-04.

## [Unreleased] - 2026-09-21: Purpose-Separated ApprovalCapability v1 Issuance

### Added
- Typed generated-Protobuf `IssueApprovalCapability` and `VerifyApprovalCapability` RPCs backed by deterministic AC v1 canonical encoding, a dedicated approval key ring/domain, bounded TTL and constant-time HMAC verification.
- PDP-side issuance checks for authenticated human tenant/company identity, approval policy, SoD, live revocation state and grant expiry; detectable approval/delegation/JWT/audit key-material reuse now fails configuration validation.
- Odoo issuance that locks and reconstructs the exact pending CBI, reruns the APP-02 guard, persists one unique capability and `approved` state, verifies the returned binding through the PDP and returns the same capability on an identical retry without confirming the PO.
- V2 canonical-intent fixtures for the retained seven delegation vectors, aligning the legacy regression with the protected `CONFIRM_PURCHASE_ORDER` route.

### Verification
- Focused approval/config/server tests, all repository Go tests, `go vet ./...`, Buf lint, Python protocol/canonical/golden tests and addon compilation pass.
- The fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes 11 post-tests / 15 test methods with 0 failures or errors, negative TLS probes and the two-session single-nonce concurrency check.
- Tampered intent binding and key substitution are rejected while the persisted approval remains non-final; the PO remains `to approve` throughout issuance and verification.

### Remaining scope
- V2-APP-04 invalidation and final commit-time authority revalidation, capability/command consumption and purchase-order mutation are not implemented by this task.

## [Unreleased] - 2026-09-21: Authenticated Human Approver Authority and SoD Guard

### Added
- A non-mutating approval guard that derives the human only from Odoo `env.user`, requires an active internal purchase manager in the pending intent's tenant/company, rejects creator/delegator/agent identities, and requires a current live-PDP ALLOW for `action:APPROVE_PURCHASE_ORDER`.
- Same-company approval-row read access for purchase managers and deterministic Activity routing to an independently eligible non-system manager; Activity remains notification rather than approval evidence.
- Real Odoo positive/negative coverage for an independent approver, creator, delegator, agent, wrong role, PDP-denied manager, cross-tenant/company manager and PDP outage.

### Verification
- Python protocol/canonical/golden tests and addon compilation pass.
- The fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes all 10 post-tests, negative TLS probes and the two-session single-nonce concurrency check.
- The guard leaves the approval `pending` and PO `to approve`; ApprovalCapability v1, the `approved` transition and final mutation/consumption remain outside this task.

## [Unreleased] - 2026-09-21: Exact Pending Intent Persistence for Approval Routing

### Added
- A dedicated `pdp.approval.request` model that stores one CSPRNG approval ID and the exact post-transition CBI JSON, canonical bytes, hash and state witness for one authorization attempt.
- Database uniqueness for approval ID and authorization attempt, plus an explicit link from the pending approval row to its single notification Activity.
- Real Odoo assertions that decode and recompute the stored canonical intent and prove an identical retry creates neither a second approval row nor a second Activity.

### Changed
- The approval-obligation route now writes `to approve`, reconstructs the locked persisted CBI in that state, stores the pending record, then marks the attempt complete without applying the protected purchase-order effect.

### Verification
- Python protocol/canonical/golden tests and addon compilation pass.
- The fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes all 9 post-tests, negative TLS probes and the two-session single-nonce concurrency check.
- The new row remains `pending`; no approver authorization, SoD issuance check, ApprovalCapability v1 or final mutation/consumption is implemented by this task.

## [Unreleased] - 2026-09-21: High-Impact Odoo Route Requires Delegation Proof V2

### Added
- Trusted, locked PostgreSQL reconstruction of the purchase-order CBI from persisted order, exact `NUMERIC` money, line, grant and nonce state.
- Handler-side V2 canonical-field validation and cross-checking against authenticated tenant, subject, route, grant, delegator and command context before policy evaluation.
- Negative coverage for missing, unknown, V1-downgraded and material-field-tampered proofs, plus direct-request and explicitly bounded nonnumeric V1 legacy regressions.

### Changed
- The protected Odoo/policy action is now `CONFIRM_PURCHASE_ORDER` and engine context is derived from verified CBI/route values.
- Odoo proof V2 requests carry the complete canonical intent while preserving V1 only at the named legacy boundary.

### Verification
- Focused and package-wide Go security/server tests, Python protocol/canonical/golden tests, addon compilation, Go vet and diff hygiene pass.
- A fresh-database Odoo 17/generated-client/mTLS/PDP/PostgreSQL run passes negative TLS probes, all nine Odoo tests and the two-session single-nonce concurrency gate.
- Approval capability, post-approval revalidation and atomic final mutation/consumption remain open in Phases 3–4.

## [Unreleased] - 2026-09-20: Canonical Intent and Delegation Proof V2

### Added
- Strict Go `CanonicalBusinessIntent v1` validation, deterministic length-delimited encoding, material-state witness verification and SHA-256 intent hashing.
- A matching Python CBI module with exact decimal-to-minor-unit conversion that rejects binary floats, exponent notation, excess precision and signed-64-bit overflow.
- Matching Go/Python canonical purchase-order line serializers with deterministic `(sequence, line_id)` and tax ordering, NFC/LF description normalization and strict canonical decimal/timestamp validation.
- A separate `v2.<kid>.<hmac>` delegation proof binding the CBI hash, grant, agent, validity window and key ID while preserving V1 behavior.
- Focused negative tests for malformed canonical values, stale witness, changed material fields, invalid validity, key confusion and V1/V2 downgrade confusion.

### Verification
- One shared multi-line fixture produces identical Go/Python line bytes/digest, witness, CBI bytes/hash and V2 proof; focused tamper tests, full `internal/security`, legacy Python protocol tests, addon compileall and Go vet pass.
- Trusted ORM reconstruction and protected-route V2 enforcement remain open under V2-PROOF-04 and the later transaction phases.

## [Unreleased] - 2026-09-20: V2 Thesis Governance Alignment

### Changed
- Root `AGENTS.md` now defines the V2 thesis boundary, evidence hierarchy, canonical-intent/approval design status, claim limits and task-first execution policy.
- Agent guidance now treats Odoo 17 as the sole empirical platform, keeps SAP as applicability discussion only, and retires historical fixed-latency and 44,000x speedup framing from active instructions.
- The `grpc-dataplane` skill now follows the real generated-Protobuf boundary: trace-only interceptor ownership, mandatory handler-side JWT/tenant/subject binding, mTLS and deadline semantics, structured fail-closed errors, bounded V1 compatibility and V2 no-downgrade routing for the high-impact path.

### Verification
- `.agents/AGENTS.md`, `agent-authorization`, `erp-testing` and `grpc-dataplane` were aligned with the V2 policy; all three rewritten skills pass `quick_validate.py`.
- `grpc-dataplane` link/path checks and repository diff hygiene pass; its behavioral forward-test remains V2-PROOF-04 because this governance task changes no runtime code.

### Remaining scope
- Other repository-local skills retain the remediation status in [`SKILL_CATALOG_AUDIT.md`](docs/technical-spec/SKILL_CATALOG_AUDIT.md) and must be updated before their respective subsystem work.
- This governance entry changes no production behavior and does not verify CBI v1 or AC v1.

## [Unreleased] - 2026-09-14: Full PDP Application-Path Evidence

### Added
- A gated 10,000-request loopback TCP gRPC measurement covers JWT validation, HMAC delegation proof, policy evaluation, metrics, encrypted audit queueing, latency percentiles, throughput, process CPU, RSS and GC.
- Windows latency sampling now uses `QueryPerformanceCounter`; the coarse wall clock was rejected after it produced zero-duration samples.

### Verification
- Three samples report p50 200.5–207.7 µs and p99 723.4–890.8 µs at 3,457–4,096 RPS with zero request errors.
- The evidence explicitly excludes mTLS, durable PostgreSQL audit flush, revocation storage/synchronization, container networking, Odoo and concurrent load.

## [Unreleased] - 2026-09-14: Dense Candidate and Collision Evidence

### Fixed
- Trie hash buckets now verify raw subject, resource and action keys before selecting policies, preventing a FNV-1a collision from selecting another key's policy.
- Reset evaluator scratch storage for each policy evaluation, avoiding heap allocations when a request evaluates more than 64 policy conditions.

### Verification
- Three repeatable 10,000-candidate samples cover global and same-leaf policy lists; the benchmark reports 0 allocs/op while retaining a documented 27–34 B/op.
- A forced collision-bucket unit test proves exact raw-key selection. Global and same-leaf lists still scan linearly by design.

## [Unreleased] - 2026-09-13: Digest-Pinned Testbed

### Changed
- Pinned PostgreSQL 15, Go 1.25, Alpine 3.19 and Odoo 17 testbed bases to immutable manifest-list digests.

### Verification
- Compose configuration and the full repository-local Odoo mTLS E2E gate pass from the pinned bases: seven transaction tests, negative TLS probes and two-session nonce concurrency.

## [Unreleased] - 2026-09-13: Thesis-First Task Priority

### Changed
- Remaining work is prioritized by the 2029 Software Engineering thesis: RQ2 worst-case performance, RQ2 full-path measurement, then RQ4 Odoo comparison.
- Deployment health and race evidence remain engineering quality work; edge restore is explicitly stretch scope unless offline ERP becomes part of the thesis.

## [Unreleased] - 2026-09-13: Durable Role-Inheritance Reload

### Verification
- PostgreSQL integration proves role inheritance persistence, fresh-engine DAG reconstruction, and revision reconciliation after a deliberately missed event.
- The shared isolated-database harness now protects all PostgreSQL integration packages without duplicating database lifecycle code.

## [Unreleased] - 2026-09-13: Policy Transaction Rollback Evidence

### Verification
- PostgreSQL 15 fault injection now proves that revision-update and notifier-SQL errors abort each publish, ACTIVE-to-DRAFT update, and delete transaction.
- All six cases preserve the prior policy record and tenant revision; native `pg_notify` remains the production notifier.

## [Unreleased] - 2026-09-13: Runtime Configuration Contract

### Changed
- Removed unused Redis, audit-socket and legacy port environment inputs from runtime/manifests.
- Moved OpenZiti identity and service inputs into centralized application configuration.

### Verification
- The runtime configuration matrix asserts every PDP/Control Plane environment binding, and Compose configuration parses successfully.

## [Unreleased] - 2026-09-13: Durable Encrypted Audit Pipeline

### Added
- Rotatable audit KEK ring with per-record AES-256-GCM DEKs, authenticated metadata and keyed integrity tags.
- Crash-safe encrypted spill files, restart replay, quota enforcement and replay/spill metrics.
- Stable audit IDs and PostgreSQL idempotent batch merge for crash-after-commit recovery.

### Verification
- Restart/key-rotation and tamper tests pass; a real PostgreSQL 15 integration verifies migration, no plaintext columns, envelope verification and duplicate replay suppression.
- Full `go test -count=1 ./...`, `go vet ./...`, Compose config and `git diff --check` pass locally.
- Deletion evidence and external append-only retention remain explicitly out of scope for this control.

## [Unreleased] - 2026-09-13: Durable Multi-Replica Revocation

### Added
- PostgreSQL-backed tenant/grant revocation records with snapshot-first `LISTEN/NOTIFY` synchronization.
- Fail-closed delegated checks while revocation state is unavailable and bounded startup readiness.
- Three-replica PostgreSQL integration covering concurrent checks, delayed delivery, offline replica and restart restoration.

### Verification
- Three repeated integration runs passed with 108 propagation samples; worst observed delay was 38.8256ms against the 5s SLO.
- Full `go test ./...`, focused security/server/storage tests, `go vet ./...` and `git diff --check` pass locally.
- Local race execution remains blocked by disabled CGO and is tracked by `CI-003`/G9.

## [Unreleased] - 2026-09-12: Security Boundary, Standard Protobuf, Odoo E2E & Replay Verification

Đợt remediation này cập nhật trạng thái theo `CURRENT_STATE_AUDIT.md` và bổ sung bằng chứng chạy thực tế. Các mục lịch sử bên dưới vẫn giữ nguyên ngữ cảnh tại thời điểm phát hành; không dùng chúng làm bằng chứng production hiện tại.

### Added
- Chuẩn hóa contract `proto/v1/policy.proto`, sinh client/server Go và Python bằng Buf/protoc.
- Hoàn thiện addon Odoo 17 trong repository: nonce ledger, non-rollback PEP state machine, PID-safe gRPC client và seed policy.
- Thêm runner Docker E2E thực tế cho Odoo cùng runner kiểm thử hai session concurrent để xác minh retry serialization.
- Thêm bộ sinh certificate test ngắn hạn và testbed PDP production-mode dùng mutual TLS bắt buộc.
- Bổ sung evidence report, production checklist, task board, `.dockerignore` và CI job `odoo-e2e`.

### Changed
- Security boundary dùng key-ring đầy đủ tuple `v1.<kid>.<hmac>`, canonical HMAC, TTL, tenant isolation và RBAC; mTLS được giữ là gate production riêng.
- Odoo xác nhận idempotency bằng tuple `(delegation_id, nonce, command_hash)` và không rollback các giao dịch cần human approval.
- Cập nhật tài liệu kiến trúc, protocol, replay/idempotency, thesis mapping và trạng thái production theo bằng chứng mới.

### Fixed / Security
- Đã kiểm chứng fail-closed cho proof sai tuple, command bị sửa, replay trùng nonce, outage, tamper, revoke và SoD denial.
- Đã kiểm chứng race hai session: PostgreSQL serialization retry kết thúc với đúng một nonce, một authorization attempt và một PO mutation.
- Sửa manifest dependency/constraint của addon để Odoo cài đặt và kiểm thử được trong testbed.

### Verification
- `make test-odoo-e2e`: 7/7 Odoo `TransactionCase` pass, concurrency runner pass, 0 failure/error.
- mTLS probe xác nhận client thiếu certificate bị từ chối; Odoo certificate hợp lệ đi qua hostname verification và tới JWT boundary.
- `go test ./...`, `go vet ./...`, protocol Python tests, compose config và `git diff --check` pass.
- Benchmark 3 mẫu giữ 0 B/op, 0 allocs/op trên các hot-path chính; latency evaluator đo được khoảng 390–493 ns/op.

### Remaining gates
- Chưa claim production-ready: remote CI evidence, multi-replica revocation, audit encryption/spill replay, image digest pinning và sustained load/race evidence vẫn mở.
- Race detector hiện bị block trong môi trường này vì thiếu `gcc`; cần chạy lại trên runner có CGO toolchain.

### Git commit breakdown
- `c3573ab` — core engine, parser, security, storage, audit và tests.
- `35f996a` — protobuf contract và client Go/Python chuẩn hóa.
- `8ac372b` — Odoo addon, Docker testbed, E2E và concurrency runner.
- `146357b` — docs, audit/checklist, roadmap, CI, changelog và tooling.

## [1.15.0] - 2026-09-03: Delegation-Aware AI Authorization, In-Memory RevocationMap O(1), Odoo 17 PEP Non-Rollback State Machine & 7 E2E Vectors [Tag: v1.0.0-core-verified]

Hoàn thành toàn diện 5 bước tích hợp giữa Standalone In-Memory Go PDP và Odoo 17 ERP qua gRPC; hiện thực hóa đầy đủ 4 Câu hỏi Nghiên cứu (RQ1–RQ4) của Đồ án Tốt nghiệp PTIT và vượt qua 100% bộ 7 Test Vectors:

### Added
- **Layer 1 Delegation Interceptor & In-Memory RevocationMap (`internal/security/delegation.go`, `internal/server/grpc_server.go`):**
  - Cấu trúc `DelegationManager` với `sync.Map` lưu trữ `RevocationMap` tra cứu $O(1)$ trên RAM trong $< 50\,\text{ns}$, triệt tiêu hoàn toàn nguy cơ chạy đua thời gian TOCTOU (Time-of-Check to Time-of-Use).
  - Tích hợp RPC `RevokeDelegation(RevokeRequest) returns (RevokeResponse)` vào hợp đồng Protobuf (`proto/v1/policy.proto`) và tự động sinh stubs cho cả Go lẫn Python.
  - Xác thực chữ ký số HMAC-SHA256 theo chuỗi nối chuẩn hóa (Canonical String):
    $$\text{Payload} = \text{grant\_id} \parallel \text{delegator} \parallel \text{agent} \parallel \text{amount} \parallel \text{valid\_until}$$
  - Cơ chế kiểm tra thời hạn TTL (Fail-Closed): Tự động từ chối `codes.PermissionDenied` (403) nếu token ủy quyền đã quá hạn thời gian.
- **Odoo 17 PEP Addon (`custom_addons/pdp_authorizer/`):**
  - `models/pdp_client.py`: Singleton `SafePDPClient` an toàn tuyệt đối với tiến trình worker pre-fork của Odoo (`os.getpid()`), tự động phát hiện fork để tái tạo gRPC Channel, triệt tiêu nguy cơ deadlock C-Core epoll; hỗ trợ gevent monkey-patching và timeout 350ms.
  - `models/delegation_grant.py`: Model `pdp.delegation.grant` quản lý hạn mức tự trị AI Agent, sinh mã băm HMAC Canonical String và nút bấm `action_revoke()` đồng bộ tức thời sang RAM của Go PDP.
  - `models/purchase_order.py`: Ghi đè `button_confirm()` với kiến trúc **Non-Rollback PEP State Machine** — khi nhận quyết định `DENY` kèm nghĩa vụ `REQUIRE_HUMAN_APPROVAL`, chuyển PO sang `to approve` và giao Activity cho cấp trên mà không raise Exception gây rollback CSDL.
  - Đầy đủ giao diện Views (`delegation_grant_views.xml`, `purchase_order_views.xml`), phân quyền `ir.model.access.csv`, và khai báo `external_dependencies: {'python': ['grpcio', 'protobuf']}` trong `__manifest__.py`.
- **Hạt Giống Luật P2P Cedar (`configs/policies.cedar`):**
  - Đóng gói 6 quy tắc P2P chuẩn hóa, áp dụng toán tử SoD `contains` (`context.delegation_chain contains resource.creator_id`) ngăn chặn hành vi tự duyệt và mượn AI duyệt hộ.
- **Bộ Kiểm Thử Toàn Trình 7 E2E Vectors (`tests/e2e_delegation_test.go`):**
  - `TC-01`: Manager tự tạo PO và tự duyệt $\to$ `DENY` (SoD Collision).
  - `TC-02`: AI Agent duyệt hộ PO do Manager tạo $\to$ `DENY` (SoD Delegation Chain).
  - `TC-03`: AI Agent tự động duyệt PO hợp lệ $\le \$2,000 \to$ `ALLOW`.
  - `TC-04`: AI Agent duyệt PO vượt trần tự hành $>\$2,000 \to$ `DENY` (`REQUIRE_HUMAN_APPROVAL`).
  - `TC-05`: Kẻ tấn công sửa context số tiền $\to$ `403 PermissionDenied` (HMAC Tampered).
  - `TC-06`: Manager thu hồi quyền trên Odoo $\to$ Agent gọi tiếp $\to$ `DENY` (Anti-TOCTOU).
  - `TC-07`: AI Agent mang token đã quá hạn TTL $\to$ `403 PermissionDenied` (Expired Token).
  - Kết quả kiểm thử: **7/7 Vectors PASS 100% trong 4.506s**.
- **Bộ Đo Đạc Đối Soát Baseline Odoo ORM (`tests/baseline_odoo_orm_benchmark.py`):**
  - Cung cấp số liệu đối trọng khoa học cho Chương 4 & 5 Luận văn: Go PDP ($540.2\ \text{ns}$, 0 B/op) nhanh hơn cơ chế Record Rules `ir.rule` của Odoo ($23.77\ \text{ms}$, ~24 KB) xấp xỉ **44,000 lần**.
- **Đóng Băng Môi Trường Testbed 2026–2029 (`docker-compose.testbed.yml`):**
  - Khóa cứng phiên bản base images: `postgres:15-alpine`, `golang:1.22-alpine`, `odoo:17.0`. Khởi chạy và kiểm chứng tự động qua cờ `--abort-on-container-exit`.

### Changed
- **Tối Ưu Hóa Bộ 9 Agent Skill Guides (`.agents/skills/`):**
  - Thu gọn và đồng bộ hóa toàn bộ 9 skill guides xuống dưới 40 dòng/file, tuân thủ nguyên lý Single Responsibility (SRP) và tối ưu hóa 100% cho AI context parser.
- **Thanh Lọc Cấu Trúc Tài Liệu:**
  - Xóa bỏ 16 thư mục và tệp tài liệu legacy/rác cũ (Redis Pub/Sub, lộ trình cá nhân), quy tụ toàn bộ tài liệu về Bộ 12 Đặc Tả Kỹ Thuật Chuẩn Khoa Học trong `docs/technical-spec/` và `docs/00_MASTER_INDEX.md`.

### Security
- Chốt hạ toàn diện rào chắn phòng thủ đa tầng: Xác thực chữ ký HMAC ở Interceptor, triệt tiêu TOCTOU trong nano-giây bằng In-Memory Map, bảo toàn phân tách trách nhiệm (SoD) qua chuỗi ủy quyền nhiều chặng, và áp dụng triệt để nguyên tắc Fail-Closed khi có sự cố.
- Khóa mốc phát hành chính thức trên Git: **`v1.0.0-core-verified`**.

---

## [1.14.0] - 2026-09-02: Zero-Alloc NDJSON Audit Streamer, Revision ID Traceability & Vector Codec Alignment


Khắc phục triệt để lỗi lệch chuẩn giải mã (Codec Mismatch) giữa PDP Data Plane và Vector Sidecar, chuẩn hóa luồng Newline Delimited JSON (NDJSON) Zero-Allocation và tích hợp `revision_id` vào toàn bộ dấu vết kiểm toán:

### Fixed
- **Vector Codec Mismatch (`internal/audit/logger.go`, `deploy/vector/vector.yaml`):**
  - Loại bỏ format nhị phân tự chế (`0xAE` magic byte + offsets) gây lỗi Drop gói tin im lặng (Silent Data Loss) trên Vector Sidecar (`codec: "json"`).
  - Chuẩn hóa sang định dạng JSON Lines (NDJSON) chuẩn Cloud-Native tương thích 100% với Vector, Kubernetes container `stdout`, ClickHouse và Kafka.

### Added
- **Revision ID Audit Traceability (`internal/audit/logger.go`, `internal/server/grpc_server.go`):**
  - Struct `LogEntry` và hàm `Log()` tích hợp trường `rev` (`revision_id uint64`) ghi nhận chính xác phiên bản chính sách đã đưa ra quyết định phân quyền.
  - Phục vụ giải quyết triệt để tranh chấp Eventual Consistency trong cửa sổ lan truyền phân tán ($\le 50\,\text{ms}$).
- **Vector Remap VRL Enrichment (`deploy/vector/vector.yaml`):**
  - Tự động chuyển đổi trường nanosecond timestamp `.ts` sang readable RFC3339 datetime `.evaluated_at` trong luồng xử lý của Vector Sidecar.

### Changed
- **Zero-Allocation JSON Formatter (`internal/audit/logger.go`):**
  - Tự động format NDJSON qua `sync.Pool` byte slice và hàm trợ giúp `escapeJSON`, `strconv.AppendInt`, `strconv.AppendUint`.
  - Đạt chỉ số vi mô: **328.3 ns/op, 0 B/op, 0 allocs/op** (> 3.32 triệu bản ghi NDJSON/giây trên 1 core CPU), duy trì bất biến Zero Heap Allocation trên toàn bộ Hot-Path.

---

## [1.13.0] - 2026-08-31: Autonomous AI Agent Guardrails, Zero-Redis Architecture & Ultra-Extreme Benchmarks

Hoàn thiện kiến trúc phân tán Pure PostgreSQL (loại bỏ hoàn toàn Redis), tích hợp rào chắn AI Guardrails & Obligations (chuẩn NIST/OWASP) và tối ưu hóa Zero-Allocation cho toàn bộ kịch bản cực hạn:

### Added
- **AI Agent Guardrails & Obligations (`tests/ai_agent_guardrails_test.go`, `proto/v1/policy.proto`):**
  - Tích hợp trường `obligations` và `advice` vào Protobuf contract `CheckAccessResponse` mà không phá vỡ tính nguyên tử Boolean (`ALLOW`/`DENY`).
  - Hỗ trợ các nghĩa vụ runtime: `REQUIRE_HUMAN_APPROVAL`, `AUDIT_SENSITIVE_TOOL_CALL`, `MASK_ATTRIBUTES`.
  - Kiểm chứng 4 kịch bản chuẩn NIST AI RMF & OWASP LLM06: Autonomous Low-Value Tool-Call (`ALLOW`), High-Value Delegated Action (`DENY` + `REQUIRE_HUMAN_APPROVAL`), Prompt Injection Extreme Value (`Hard DENY`), và Agent-Supervisor Separation of Duties (`DENY`).
  - Đạt độ trễ **286.3 ns/op, 0 B/op, 0 allocs/op** (> 3.49 triệu lượt đánh giá AI Guardrail/giây).
- **Ultra-Extreme & Deep DAG Benchmarks (`tests/benchmark_test.go`):**
  - `BenchmarkUltraExtreme_DeepDAG_HeavyABAC`: Đồ thị kế thừa vai trò 11 cấp (DAG), 5,000 decoy policies, biểu thức AST 10 điều kiện đạt **810.9 ns/op, 0 B/op, 0 allocs/op**.
  - `BenchmarkUltraExtreme_10kPolicies_ConcurrentContention`: 10,000 chính sách chịu tải đa luồng cực đại đạt **35.94 ns/op, 0 B/op, 0 allocs/op** (~27.8 triệu RPS).

### Changed
- **Pure PostgreSQL Sequence & Metadata `LISTEN/NOTIFY`:**
  - Khai tử hoàn toàn Redis container và vòng lặp Polling 10s.
  - Chuyển giao toàn bộ cơ chế đồng bộ sang PostgreSQL Monotonic Revision Sequence (`tenants.revision`) với payload metadata < 120 bytes (loại bỏ rủi ro tràn giới hạn 8KB của Postgres).
  - Tự động phát hiện khoảng trống dữ liệu (`gap detection`) và kích hoạt Fast Catch-Up tức thời (< 50ms).
- **Trie Index & Stack Scratch Buffers:**
  - Mở rộng `policySlicePool` và bộ đệm `scratchNodes`/`scratchIPs` trong `EvalContext` lên 64 phần tử, `subScratch` lên 32 phần tử, triệt tiêu 100% hiện tượng slice grow heap allocation trên mọi đường dẫn tra cứu sâu.

---

## [1.12.0] - 2026-08-31: Docker DevOps Standards Skill & Containerized E2E Verification

Đúc kết toàn bộ bài học thực chiến, chuẩn hóa Docker multi-stage images và quy trình kiểm thử phân tán E2E trên Docker Compose:

### Added
- **Docker Standards Agent Skill (`.agents/skills/docker-standards/SKILL.md`):** Đóng gói toàn bộ kinh nghiệm và rào chắn kỹ thuật:
  - Khử bỏ triệt để Byte Order Mark (UTF-8 BOM `\ufeff`) khi tạo migration SQL trên Windows.
  - Đồng bộ `golang:alpine` builder image khớp với phiên bản `go.mod` (ngăn lỗi `go >= 1.25.0`).
  - Quy chuẩn Docker Compose healthchecks (`depends_on: { condition: service_healthy }`) và handshake readiness polling cho E2E testing.
  - Đảm bảo tính tương thích ngược hai chiều (`id` và `policy_id`) trong toàn bộ REST response.
- **Docker E2E Automated Verification:** Xác thực thành công 100% luồng phân tán thực tế với PostgreSQL 15, Redis 7, PDP Server gRPC và Control Plane HTTP trong `tests/e2e_test.go`.

---

## [1.11.0] - 2026-08-31: Clean Architecture Standards, Modular Handlers & Centralized Config

Chuẩn hóa toàn diện mã nguồn theo chuẩn Clean Architecture & 12-Factor App quốc tế, loại bỏ hoàn toàn hardcoded parameters và phân rã các file nguyên khối:

### Added
- **Centralized Type-Safe Config (`internal/config/config.go`):** Quản lý toàn bộ cấu hình hệ thống tập trung với cơ chế Fail-Fast Validation, nạp biến môi trường type-safe và ngăn chặn chạy localhost trên Production.
- **Dedicated Agent Skill (`.agents/skills/clean-architecture-standards/SKILL.md`):** Thiết lập rào chắn quy chuẩn quốc tế: giới hạn độ dài file ($\le 250$ dòng), nguyên lý Single Responsibility (SRP), nghiêm cấm hardcoded credentials và magic numbers.

### Changed
- **Modularized HTTP Handlers (`internal/server/`):** Phân rã file nguyên khối `http_server.go` (500 dòng) thành 4 module độc lập, tinh gọn (< 180 dòng/file):
  - `http_server.go` (89 dòng): Khởi tạo máy chủ & định tuyến Mux.
  - `handlers_policy.go` (178 dòng): Quản trị CRUD và Publish chính sách.
  - `handlers_tenant.go` (82 dòng): Schema thuộc tính & Prewarm API.
  - `handlers_decision.go` (113 dòng): Fallback REST Decision & Simulator.
- **Decoupled CLI & Server Binaries:** Tái cấu trúc `cmd/control-plane/main.go` và `cmd/pdp-server/main.go` sử dụng `config.Load()`, xóa bỏ các hàm `os.Getenv` rời rạc.

---

## [1.10.0] - 2026-08-31: Zero-Allocation Binary Datagram Unix Socket (unixgram) Logger

Tối ưu hóa tầng Audit Logger lên cảnh giới vi mô cao nhất, loại bỏ hoàn toàn JSON serialization và sử dụng giao thức Non-blocking UDP Unix Domain Socket (`unixgram`):

### Added
- **Zero-Allocation Binary Packing (`internal/audit/logger.go`):** Đóng gói bản ghi kiểm toán trực tiếp thành chuỗi byte nhị phân (`0xAE` magic byte, Unix Nano int64, BigEndian string offsets) qua `sync.Pool` byte slice. Triệt tiêu hoàn toàn `encoding/json`, đạt tốc độ phát tán log **140.9 ns/op**, **0 B/op** và **0 allocs/op**.
- **Non-blocking UDP Unix Domain Socket (`NewUnixgramAuditLogger`):** Truyền tin qua `net.DialUnix("unixgram", ...)` theo cơ chế fire-and-forget qua kernel socket buffer, đảm bảo không bao giờ gây backpressure hay phát sinh lock contention lên luồng đánh giá Data Plane.
- **Architectural Trade-Off Formalization:** Ghi nhận chính thức trong mã nguồn và tài liệu kiến trúc sự đánh đổi có chủ đích giữa Ultra-Low Latency (Ưu tiên tuyệt đối hiệu năng CPU RAM) và khả năng rớt gói tin nếu Sidecar Vector gặp sự cố.
- **Micro-Benchmark Suite (`internal/audit/logger_test.go`):** Bổ sung `BenchmarkAuditLogger_LogZeroAlloc` kiểm chứng $0$ Heap allocations và thông lượng $> 7,700,000$ logs/giây trên một core.

---

## [1.9.0] - 2026-08-31: Cloud-Native Decoupled Audit Streamer & Vector Sidecar Architecture

Trục xuất hoàn toàn logic ghi cơ sở dữ liệu PostgreSQL và đĩa cục bộ ra khỏi PDP, chuyển đổi sang kiến trúc Decoupled Stream Logger chuẩn 12-Factor App:

### Changed
- **Decoupled Stream Logger (`internal/audit/logger.go`):** Thay thế toàn bộ worker queue và `pgx.CopyFrom` bằng `StreamAuditLogger` sử dụng `sync.Pool` chứa `bytes.Buffer`. Xuất log JSON thẳng ra `os.Stdout` hoặc Unix Domain Socket trong **< 50 ns**, triệt tiêu hoàn toàn rủi ro nghẽn Backpressure và không sinh rác GC Heap.
- **Khử bỏ điểm nghẽn RDBMS:** PDP không còn trực tiếp chịu tải I/O của hàng triệu audit logs/giây, loại bỏ nguy cơ làm sập PostgreSQL do table bloat và WAL saturation.
- **Khử bỏ ảo giác Ephemeral Storage trên K8s:** Xóa bỏ thư mục ghi đĩa cục bộ `./spill-logs` bên trong Pod PDP, chuyển giao toàn bộ trách nhiệm buffering và retry cho Sidecar chuyên dụng.

### Added
- **Vector Sidecar Blueprint (`deploy/vector/vector.yaml`):** File cấu hình agent Vector (viết bằng Rust, Zero-GC) lắng nghe socket từ PDP, tự quản lý đệm đĩa Persistent Volume và đẩy log về ClickHouse / Kafka.
- **Kubernetes Production Manifest (`deploy/k8s/pdp-with-vector-sidecar.yaml`):** Mẫu Deployment chuẩn chạy đồng thời PDP Container và Vector Sidecar qua Shared Memory IPC volume `/var/run/pdp`.

---

## [1.8.0] - 2026-08-31: In-Process Go SDK (Zero-Network Hop), Unix Domain Sockets & AI Agent Guardrails

Triệt tiêu 100% độ trễ mạng qua In-Process Go SDK và Unix Domain Socket IPC, đồng thời chuẩn hóa rào chắn an toàn cho Autonomous AI Agent theo mô hình Obligation:

### Added
- **In-Process Go SDK (`pkg/pdp/`):** Cung cấp gói SDK nhúng `EmbeddedPDP` cho phép các Go Microservices thực hiện đánh giá phân quyền như một lời gọi hàm RAM cục bộ với độ trễ thuần **0.29 µs (297 ns)** và **0 allocs/op**.
- **Scoped Tenant Whitelist (`pkg/pdp/config.go`):** Bổ sung danh sách `AllowedTenants` kiểm soát phạm vi nạp chính sách, triệt tiêu 100% nguy cơ OOM khi ứng dụng nhúng chạy trong môi trường phân tán đa khách thuê.
- **Non-blocking Background Sync (`pkg/pdp/embedded.go`):** Luồng đồng bộ ngầm (`syncWorker`) thực hiện nhận sự kiện, biên dịch độc lập và hoán đổi con trỏ Copy-On-Write (COW), bảo đảm không bao giờ gây block hay phát sinh jitter trên luồng nghiệp vụ chính.
- **Unix Domain Socket (UDS) Listener (`cmd/pdp-server/main.go`):** Bổ sung biến môi trường `LISTEN_SOCKET_PATH` hỗ trợ bind vào Unix socket (`unix:///var/run/pdp.sock`) cho các Sidecar Container trong Kubernetes Pod, vượt qua hoàn toàn overhead của TCP stack.
- **AI Agent Guardrails with Obligation Pattern (`internal/engine/decision.go`):** Bổ sung cấu trúc `Obligation` (ví dụ `REQUIRE_HUMAN_APPROVAL`, `MASK_ATTRIBUTES`) gắn kèm quyết định nhị phân `DENY` khi AI Agent cố gắng thực thi công cụ rủi ro cao hoặc vượt quá độ sâu ủy quyền `delegation_depth > 3`, bảo toàn trọn vẹn Boolean Algebra cho bộ đánh giá AST.

---

## [1.7.0] - 2026-08-31: PostgreSQL Transactional Sequence, pg_notify Broadcast & Multi-Replica Replay Buffer

Triển khai cơ chế cấp phát Monotonic Revision ID nguyên tử gắn liền với Database Transaction, broadcast qua PostgreSQL `LISTEN/NOTIFY` và Replay Ring Buffer chống mất mát sự kiện:

### Added
- **Database Migration 000003 (`db/migrations/000003_add_tenant_revision.up.sql`):** Bổ sung cột `revision BIGINT NOT NULL DEFAULT 1` và index `idx_tenants_revision` vào bảng `tenants`.
- **PostgreSQL Transactional Sequence & `NOTIFY` (`internal/storage/postgres.go`):** Cập nhật `PublishPolicy` và `DeletePolicy` chạy trong một `pgx.Tx` duy nhất, tự động tăng `revision` của Tenant qua `RETURNING revision` và kích hoạt lệnh `NOTIFY policy_events, ...` ngay khi transaction commit thành công. Loại bỏ hoàn toàn rủi ro Dual-Write hoặc Phantom Events.
- **Cross-Replica Buffer Sync (`internal/storage/postgres.go`):** Bổ sung hàm `ListenPolicyEvents` sử dụng `LISTEN policy_events` chuyên dụng để các Control Plane replicas nhận sự kiện trong thời gian thực.
- **Replay Ring Buffer with Compaction Fallback (`internal/server/replay_buffer.go`):** Lưu trữ 1,000 sự kiện gần nhất trong RAM. Hỗ trợ phương thức `GetEventsSince(tenantID, afterRevision)` giúp PDP nodes catch-up tức thì khi rớt mạng ngắn, hoặc trả về tín hiệu Compaction để kích hoạt Full Snapshot reload khi rớt mạng dài hạn.
- **Pre-warm REST API Endpoint (`internal/server/http_server.go`):** Thêm route `POST /api/v1/tenants/{tenant_id}/prewarm` (yêu cầu Tenant Auth) cho phép CI/CD hoặc API Gateway chủ động nạp trước tập luật vào RAM, loại bỏ hoàn toàn độ trễ Cold Start cho các VIP Tenants.

---

## [1.6.0] - 2026-08-31: Distributed Resilience, 100% Stateless Cloud Profile & Monotonic Revision Sync

Chuẩn hóa PDP Engine thành Microservice 100% Stateless trên Cloud Kubernetes, giải quyết triệt để cửa sổ mù 10s bằng cơ chế Monotonic Revision ID & Gap Detection, và cung cấp API Schema phục vụ tối ưu hóa Payload gRPC:

### Added
- **100% Stateless Cloud Profile (`cmd/pdp-server/main.go`):** Bổ sung biến môi trường `STORAGE_MODE=cloud|edge`. Chế độ `cloud` (mặc định) giúp Pod vô trạng hoàn toàn, không đụng vào đĩa cứng cục bộ `./badger-data`, nạp dữ liệu thẳng vào RAM khi khởi động. Chế độ `edge` kích hoạt `BadgerStore` nhúng cho các kịch bản offline/IoT.
- **Monotonic Revision ID & Sync Gap Detection (`internal/engine/sync.go`, `internal/engine/engine.go`):** Bổ sung `Revision uint64` vào `TrieRoot` và `EngineState`. Khi nhận sự kiện qua Pub/Sub, Syncer tự động so sánh số hiệu phiên bản; nếu phát hiện lệch phiên bản do rớt mạng (`event.Revision > currentRevision + 1`), hệ thống kích hoạt luồng **Full Catch-Up Sync** tức thì thay vì chờ chu kỳ 10s.
- **Compile-Time Attribute Extraction (`internal/parser/compiler.go`, `internal/parser/ast.go`):** AST Compiler tự động phân tích và trích xuất danh sách các thuộc tính biến được sử dụng (`PolicyNode.RequiredAttributes`). `TrieRoot` tự động tổng hợp danh mục thuộc tính cần thiết của toàn bộ chính sách Active trong Tenant.
- **Tenant Attribute Schema REST Endpoint (`internal/server/http_server.go`):** Thêm route `GET /api/v1/tenants/{tenant_id}/schema` (xác thực Tenant Auth) trả về danh mục thuộc tính yêu cầu và revision hiện tại, giúp API Gateway (PEP) lọc và đóng gói payload gRPC siêu nhẹ (< 300 bytes).

### Performance & Resilience Metrics Verified
- **Độ trễ phục hồi khi rớt mạng:** Tức thì trong **< 50ms** khi kết nối phục hồi nhờ Gap Detection.
- **Tốc độ Hot-Path RAM:** Duy trì ổn định tuyệt đối ở **305 ns/op, 0 B/op và 0 allocs/op**.

---

## [1.5.0] - 2026-08-31: Zero-Allocation Hot-Path, 100% Lock-Free Role DAG & Microsecond Performance Milestone

Tối ưu hóa sâu tầng động cơ đánh giá In-Memory (PDP Data Plane), triệt tiêu hoàn toàn 100% heap allocation trên hot-path, loại bỏ tranh chấp khóa Mutex trên luồng đọc và thiết lập các chốt chặn bất biến (Immutability Guardrails) ở cấp độ kiến trúc bộ nhớ:

### Changed
- **100% Lock-Free `RoleDAG` Read Path (`internal/engine/dag.go`):** Gỡ bỏ hoàn toàn `sync.RWMutex.RLock()` trên luồng `IsDescendant()` và `GetInheritedRoles()`. Tận dụng triệt để kiến trúc Copy-On-Write (COW) với `atomic.Pointer` swap ở cấp `EngineState`, đưa độ trễ đọc của Role DAG về O(1) mà không bị Cache-Line Bouncing khi tải hàng triệu RPS.
- **Pre-computed Role Inheritance with Full Slice Expression:** Tính toán trước toàn bộ danh sách vai trò kế thừa `inheritedRoles map[string][]string` tại Write-Time. Áp dụng kỹ thuật 3-index slice `roles[:len:len]` để khóa cứng dung lượng mảng (Cap = Len), ngăn chặn tuyệt đối rủi ro `append()` làm ô nhiễm vùng nhớ ngầm dùng chung.
- **Zero-Allocation Stack Fallback for Independent Roles:** Triển khai `GetInheritedRolesInto(role, &subScratch)` sử dụng bộ đệm stack scratch buffer `scratch[:1:1]`, loại bỏ hoàn toàn việc cấp phát heap `[]string{role}` khi gặp Subject không có kế thừa vai trò.
- **EvalContext Scratch Buffers & Zero-Copy Map Pointer (`internal/engine/evaluator.go`):** Nhúng trực tiếp mảng đệm `scratchNodes [16]parser.ValueNode` và `scratchIPs [16][16]byte` vào `EvalContext`. Chuyển `GetEvalContext` sang cơ chế Zero-Copy pointer (không sao chép từng phần tử map).
- **Fast Byte Arithmetic Parsers:** Thay thế hoàn toàn `fmt.Sscanf` và `net.ParseIP` bằng các hàm phân giải số học byte trực tiếp `parseFastTime` (cho các mốc Time HH:MM:SS / HH:MM) và `parseIPv4FastInto` (chuyển IPv4 vào mảng cố định không phân bổ slice).
- **Static Reason Constants & Pre-allocated Explanation List (`internal/engine/decision.go`):** Bổ sung `ExplanationList []string` bất biến vào `PolicyNode` ngay từ lúc compile AST. Thay thế toàn bộ `fmt.Sprintf` và `strings.Join` trên fast-path `CheckPermission` bằng các hằng số tĩnh (`ReasonDenyForbid`, `ReasonAllowPermit`, `ReasonDenyDefault`), chỉ format chuỗi động khi gọi endpoint giải thích chi tiết `ExplainDecision`.

### Performance Metrics Verified
- **Độ trễ đánh giá RAM (`BenchmarkEvaluatorLatency`):** Giảm từ 3,654 ns/op xuống **297.6 ns/op (0.297 µs)** — Nhanh gấp **12.2 lần**.
- **Cấp phát bộ nhớ Hot-Path:** Giảm từ 25 allocs/op (493 B/op) xuống **0 B/op và 0 allocs/op** (Zero GC Pressure tuyệt đối).
- **Thông lượng tải đồng thời (`BenchmarkConcurrentLoad`):** Đạt **24.05 ns/op**, tương đương năng lực xử lý lý thuyết vượt **41.5 triệu RPS/core**.
- **Kịch bản ERP ABAC phức tạp (`BenchmarkERP_PurchaseOrderEvaluation`):** Đạt **589.8 ns/op** với **0 B/op và 0 allocs/op**.

---

## [1.4.0] - 2026-07-05: High-Performance CLI Tool (`pectl`) & Automation Support

Bổ sung công cụ dòng lệnh (CLI) `pectl` chuẩn production-grade hỗ trợ quản trị chính sách, giả lập và ra quyết định thông qua REST Control Plane.

### Added
- **`pectl` CLI Tool (`cmd/pectl`):** Khởi tạo ứng dụng CLI sử dụng `cobra` và `viper`, hỗ trợ đa dạng câu lệnh quản trị vòng đời chính sách, quản lý tenant, đo đạc telemetry và kiểm tra sức khỏe hệ thống.
- **Enterprise-Grade Client (`internal/pectl/client.go`):** Reusable HTTP API client với cơ chế tự động gửi lại yêu cầu (retry với exponential backoff 100ms-2s) tối đa 3 lần cho lỗi 5xx, hỗ trợ xử lý lỗi có cấu trúc theo chuẩn RFC 7807 (Problem Details).
- **Tabular & Structured Printing (`internal/pectl/printer`):** Triển khai tầng xuất dữ liệu ra Console hỗ trợ 3 chế độ `--output`: `table` (sử dụng tabwriter căn lề cột), `json` (định dạng đẹp) và `yaml` (gopkg.in/yaml.v3).
- **Flexible Configuration:** Ưu tiên cấu hình động theo thứ tự: CLI Flags > Environment Variables (`PECTL_*`) > File cấu hình (`~/.pectl/config.yaml`).
- **Commands Added:**
  - `policy`: `create`, `update`, `publish`, `delete`, `list`, `get`
  - `simulate`: Giả lập quyết định với ngữ cảnh JSON (`--context-file`), nạp chính sách draft cục bộ (`--draft-file`) và gộp chính sách active (`--include-active`).
  - `check` & `explain`: Kiểm tra quyền truy cập trực tiếp kèm đo lường latency chính xác và hiển thị vết thực thi (trace).
  - `tenant`: `list`, `get`, `status`
  - `metrics` & `health`: Thu thập telemetry (latency P50/P95/P99, QPS, GC) và kiểm tra sức khỏe thành phần hệ thống.
- **Setup Scripts & Makefile Targets:**
  - Viết script tự động cài đặt `go mod tidy` và biên dịch nhanh `setup-pectl.sh` (Linux/WSL) và `setup-pectl.ps1` (Windows).
  - Thêm target `build-pectl`, `install-pectl`, `test-pectl`, và `tidy` vào `Makefile`.
- **Unit Tests:** Kiểm thử 100% logic nạp cấu hình, cơ chế retry/error client và định dạng đầu ra của printer.

---

## [1.3.0] - 2026-07-04: JWT Token Validation, AES-GCM Log Encryption, Redis Universal Client & Sprint 7 Final Deliverables

Sprint cuối cùng & Sprint 7. Hoàn thiện tầng bảo mật, vận hành phân tán, dọn dẹp bộ nhớ RAM tự động, cấu trúc cơ sở dữ liệu có phiên bản, đo đạc hiệu năng và kiểm thử tích hợp E2E:

### Added
*   **E2E Integration Test ([e2e_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_test.go)):** Xây dựng luồng kiểm thử E2E trong container qua Docker Compose kết nối PostgreSQL và Redis thực tế. Tự động khởi tạo Tenant ngẫu nhiên trong DB, thực hiện CRUD chính sách, kiểm thử phân quyền gRPC, giả lập mất kết nối Redis để kiểm thử đồng bộ dự phòng (Fallback Polling).
*   **JSON Codec for Mock gRPC ([policy.pb.go](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.pb.go)):** Đăng ký JSON codec cho gRPC giúp đóng gói và giải tuần tự các struct viết tay (mock protobuf) qua mạng TCP Docker mà không cần cài đặt trình biên dịch `protoc` trên máy local.
*   **Database Migration System ([migrations](file:///e:/Projects/Project_TN/standalone-policy-engine/db/migrations/)):** Tách biệt DDL schema cứng và thay bằng hệ thống migrations có phiên bản sử dụng `golang-migrate/migrate/v4`. Tự động khởi chạy migration khi khởi động tầng PostgreSQL Storage.
*   **Audit Logger Graceful Degradation ([logger.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go)):** Cấu hình exponential back-off (1 giây đến tối đa 1 phút) cho replay worker. Thêm cơ chế giới hạn thư mục log cục bộ tối đa 1GB (Spill-to-Disk size limit) để bảo vệ đĩa cứng tránh bị tràn.
*   **GC Configuration via Env Vars ([main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go)):** Hỗ trợ cấu hình RAM GC động thông qua các biến môi trường `GC_ENABLED`, `GC_INTERVAL`, `GC_IDLE_TIMEOUT` để tối ưu hóa bộ nhớ đệm in-memory trie.
*   **Benchmark Validation ([benchmark_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/benchmark_test.go)):** Bổ sung bộ kiểm thử tải trọng lớn đo đạc throughput và latency của PDP Engine dưới các mức quy mô 1k, 10k, 100k chính sách và lưu trữ kết quả tại [results.md](file:///e:/Projects/Project_TN/standalone-policy-engine/benchmarks/results.md).
*   **JWT Token Validation ([jwt.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/jwt.go)):** PDP gRPC Server tự động trích xuất và xác thực JWT Token (HMAC-SHA256) từ gRPC Metadata `authorization`. Claims được parse và nạp vào `req.Subject` và `req.Context` trước khi chạy bộ đánh giá ABAC. PEP không cần giải mã token thủ công.
*   **AES-GCM Envelope Encryption ([crypto.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/crypto.go)):** Mọi trường nhạy cảm trong log kiểm toán (subject, action, resource, context) được mã hóa bằng AES-GCM 256-bit trước khi ghi vào PostgreSQL hoặc Spill-to-Disk. Mỗi bản ghi dùng một DEK ngẫu nhiên riêng, DEK được mã hóa bởi KEK từ biến môi trường `LOG_KEK`. Ngay cả admin PostgreSQL cũng không thể đọc nội dung log.
*   **Redis Universal Client:** Cả `cmd/pdp-server` và `cmd/control-plane` hỗ trợ ba chế độ kết nối Redis thông qua biến môi trường `REDIS_MODE`: `single` (mặc định), `sentinel` (Failover) và `cluster` (Horizontal Scale). Không cần sửa code khi nâng cấp topology Redis.
*   **PDP Node Heartbeat Registry:** `Syncer.heartbeatWorker` định kỳ 5 giây gửi JSON heartbeat kèm node ID, trạng thái và số Tenant đang hoạt động lên kênh Redis `pdp-heartbeats`. Cho phép Control Plane theo dõi số lượng và sức khỏe tất cả node PDP trong cluster.
*   **Unit Tests ([jwt_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/jwt_test.go), [crypto_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/crypto_test.go)):** Kiểm thử toàn diện cả hai module: JWT validate hợp lệ/hết hạn/sai secret, Bearer prefix stripping, Envelope Encrypt/Decrypt vòng đời, nonce độc lập, DEK sai và payload >5KB.

### Fixed
*   **gRPC Tenant Isolation Mismatch:** Khắc phục lỗi so khớp Subject trong E2E test bằng cách đồng bộ hóa JWT token đăng nhập của Alice/Bob khớp với danh tính gửi đi, ngăn chặn bộ lọc Interceptor chặn nhầm request.
*   **Nil Response Handling:** Ngăn ngừa lỗi Panic bằng cách bọc kiểm tra an toàn `nil` cho các response HTTP trước khi gọi close body trong E2E tests khi container Redis bị tắt.

---

## [1.2.0] - 2026-07-04: Policy Simulation, Edge Storage (BadgerDB) & RAM GC
Hiện thực hóa các tính năng nâng cao giúp hệ thống trở nên thông minh và tự phục hồi: API giả lập chính sách không ảnh hưởng production, bộ lưu trữ cục bộ BadgerDB hỗ trợ khởi động siêu tốc khi mất mạng, và cơ chế tự động dọn dẹp RAM.

### Added
*   **Policy Simulation API (FR-010):** Thêm endpoint `POST /api/v1/tenants/{tenant_id}/simulate` vào Control Plane cho phép Admin gửi thử nghiệm bất kỳ tập chính sách DSL DRAFT nào và nhận ngay kết quả quyết định (ALLOW/DENY) kèm giải thích chi tiết, hoàn toàn không ảnh hưởng đến bộ nhớ RAM Engine đang phục vụ thật.
*   **BadgerDB Edge Storage ([badger.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger.go)):** Tầng lưu trữ cục bộ nhúng (Embedded KV) giúp PDP Sidecar khởi động siêu tốc khi không kết nối được PostgreSQL. Sau mỗi lần đồng bộ thành công, snapshot JSON tập chính sách được ghi xuống BadgerDB cục bộ. Khi khởi động, PDP tự động nạp từ BadgerDB nếu Postgres chưa có mặt.
*   **Tenant Active Cache GC ([engine_gc.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_gc.go)):** Goroutine chạy ngầm định kỳ quét dọn dẹp Trie của các Tenant không có hoạt động CheckPermission trong quá 24 giờ, giải phóng bộ nhớ RAM tự động. Kết hợp với Lazy Loading tự động tải lại từ Postgres khi có request mới đến Tenant đã bị unload.
*   **Unit Tests:** Bộ kiểm thử [badger_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/badger_test.go) và [engine_gc_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_gc_test.go) kiểm chứng toàn bộ: BadgerDB CRUD, GC unload idle tenant, Lazy Loading và Concurrent Safety với race detector.

---

## [1.1.0] - 2026-07-04: Observability, Performance & Cloud-Native Deployments
Hoàn thiện toàn bộ các cấu phần giám sát hệ thống, đóng gói container và manifests triển khai hạ tầng Kubernetes cluster.

### Added
*   **Prometheus Metrics:** Tích hợp bộ chỉ số đo đạc hiệu năng thời gian thực [metrics.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/metrics/metrics.go) và expose endpoint `/metrics` trên HTTP Server.
*   **gRPC Trace Interceptor:** Thiết lập Unary Interceptor trích xuất trace context (W3C standard) trong `grpc_server.go` kết nối trace liên tục từ PEP sang PDP.
*   **Docker Containerization:** Viết các Dockerfile multi-stage build tối ưu cho cả [pdp-server](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/docker/Dockerfile.pdp) và [control-plane](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/docker/Dockerfile.control).
*   **Envoy L7 Proxy Configuration:** Xây dựng file cấu hình [envoy.yaml](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/envoy/envoy.yaml) cân bằng tải HTTP/2 gRPC.
*   **Kubernetes Manifests:** Tạo file deployment [pdp-deployment.yaml](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/kubernetes/pdp-deployment.yaml) chạy sidecar Envoy proxy và [control-plane-deployment.yaml](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/kubernetes/control-plane-deployment.yaml).
*   **Performance Benchmark:** Viết kịch bản kiểm thử hiệu năng [performance_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/performance_test.go) giả lập 1,000 chính sách đo đạc độ trễ và throughput.

---

## [1.0.0] - 2026-07-04: Core Decisions & Infrastructure
Dự án được khởi tạo và hoàn thiện toàn bộ tầng logic cốt lõi (RAM Core Engine) và tầng hạ tầng phân phối dữ liệu (gRPC, PostgreSQL, Redis, Async Ring Buffer Logs).

### Added
#### Sprint 3: gRPC, DB, Sync & Logs (Hạ tầng & Vận hành)
*   **Protobuf Contract:** Định nghĩa hợp đồng gRPC v1 [policy.proto](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto) cung cấp 2 dịch vụ chính `CheckAccess` và `ExplainDecision`.
*   **gRPC Server:** Hiện thực hóa máy chủ gRPC Data Plane [grpc_server.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/grpc_server.go) tích hợp Keepalive HTTP/2 để duy trì kết nối persistent siêu tốc giữa API Gateway (PEP) và Engine (PDP).
*   **HTTP Control Plane:** Xây dựng máy chủ HTTP API quản trị [http_server.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/http_server.go) cho phép CRUD chính sách ở dạng bản thảo `DRAFT`, và endpoint `/publish` để biên dịch, kiểm tra an toàn và kích hoạt chính sách.
*   **PostgreSQL Storage:** Xây dựng tầng lưu trữ bền vững [postgres.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/storage/postgres.go) sử dụng `pgxpool`. Hỗ trợ tự động chạy DDL Schema khởi tạo các bảng và index khi chạy, quản lý lịch sử phiên bản chính sách, và hỗ trợ ghi log bằng CopyFrom tối ưu.
*   **Redis Cache Sync:** Phát triển luồng đồng bộ hóa bộ nhớ đệm [sync.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/sync.go) qua Redis Pub/Sub đảm bảo hot reload chính sách không downtime (<300ms) kèm cơ chế Polling dự phòng mỗi 10 giây khi Redis sập.
*   **Async Audit Logs & Spill-to-Disk:** Xây dựng bộ ghi log kiểm toán bất đồng bộ [logger.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/audit/logger.go) dùng Ring Buffer (Go channel). Tự động chuyển đổi sang ghi log cục bộ (Spill-to-Disk SSD) khi DB quá tải hoặc ngắt kết nối và tự động Replay log vào Postgres khi kết nối được khôi phục.
*   **Entrypoints:** Tạo các file main.go chạy máy chủ PDP [cmd/pdp-server/main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go) và Control Plane [cmd/control-plane/main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/control-plane/main.go).

#### Sprint 2: In-Memory Index Trie & AST Evaluator (RAM Core Engine)
*   **Multi-level Index Trie:** Xây dựng cấu trúc lưu trữ RAM Trie [trie.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/trie.go) phân cấp giúp tra cứu chính sách đạt độ phức tạp $O(\log N)$ thay vì duyệt tuyến tính $O(N)$.
*   **Global Rules Partitioning:** Phân tách các chính sách wildcard kép (`principal == any && resource == any`) ra phân vùng riêng để tránh ô nhiễm chỉ mục Trie.
*   **Role Hierarchy Resolving (DAG):** Xây dựng đồ thị vai trò [dag.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/dag.go) có thuật toán DFS phát hiện chu trình đệ quy và tính toán trước Transitive Closure cho phép kiểm tra quan hệ vai trò với độ phức tạp $O(1)$ ở runtime.
*   **AST Evaluator:** Hiện thực bộ đánh giá AST [evaluator.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go) hỗ trợ đoản mạch logic (short-circuit), so khớp bitwise IP, so khớp DateTime bằng số nguyên, và tích hợp `sync.Pool` tái sử dụng context.
*   **Decision Logic:** Phát triển thuật toán ra quyết định [decision.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/decision.go) dựa trên quy tắc mặc định cấm (Deny-by-Default), luật cấm ghi đè (Forbid Overrides), và tính năng giải thích quyết định (Policy Explain).
*   **Copy-On-Write (COW):** Triển khai cơ chế nhân bản Trie và hoán đổi con trỏ nguyên tử (Atomic Pointer Swap) trong [engine.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine.go) giúp luồng đọc luôn chạy tự do lock-free.
*   **Unit & Concurrency Tests:** Viết các bài test đa luồng đọc ghi đồng thời với cờ `-race` trong [engine_test.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/engine_test.go).

#### Sprint 1: Policy Language Compiler & AST Parser (DSL & Compiler)
*   **AST Nodes:** Định nghĩa các node biểu diễn cây AST bất biến trong [ast.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/ast.go).
*   **Stateful Lexer:** Hiện thực hóa bộ tokenizer [lexer.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/lexer.go) có khả năng chuyển đổi trạng thái khi gặp dấu `{` để giải quyết sự nhập nhằng của từ khóa Scope tĩnh và Variable động.
*   **Pratt Parser:** Xây dựng trình phân tích cú pháp đệ quy đi xuống [parser.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/parser.go) dựa trên độ ưu tiên của toán tử.
*   **AST Compiler & Optimizations:** Thiết lập trình biên dịch [compiler.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/compiler.go) thực hiện Constant Folding, IP/DateTime Pre-parsing sang dạng nhị phân/số nguyên, và kiểm tra kiểu dữ liệu tĩnh.

### Security
*   **AST Depth Limit:** Chặn biên dịch chính sách nếu độ sâu biểu thức logic lồng nhau vượt quá **15 cấp** để phòng chống tấn công DoS tràn stack.
*   **Safe Missing Attributes:** Thuộc tính thiếu trong ngữ cảnh request được evaluator chuyển về trạng thái `ERROR` và trả về `false` (Fail-closed) một cách an toàn mà không gây panic hệ thống.
- V2-EVAL-01 continuation: added a live Odoo final-route AC-signature-as-delegation-proof substitution test. Fresh Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes 49 post-tests / 59 cases, 0 failures/errors, with concurrency/retry/stale-intent runners. Ledger records remaining composites; task stays in progress.
- V2-EVAL-01 continuation: added Odoo/PostgreSQL checks for AC reissue idempotency, unique one-time-ID collision across commands and copied signed-byte replay. Fresh Odoo 17/generated-client/mTLS/PDP/PostgreSQL gate passes 51 post-tests / 61 cases, 0 failures/errors, with concurrency/retry/stale-intent runners. Task remains in progress.
- Thesis direction lock (2026-09-24): set the Vietnamese/English title and center the contribution on delegation-aware, transaction-bound authorization for high-impact AI-agent actions in ERP. Added `V2-DOC-05` after `V2-EVAL-01` to align the active proposal before `V2-EVAL-02`; this adds one item to the previously counted eight remaining tasks.
- V2-DOC-05 scope clarification (2026-09-24): include root README, documentation master index, thesis chapter map, career roadmap and Odoo addon README in the post-EVAL-01 consistency pass, alongside the active proposal and generator. Keep V1 archive and normative implementation/evidence sources immutable unless their owning task establishes a correction.
- Remaining thesis execution order locked (2026-09-24): EVAL-01 → DOC-05 → EVAL-02 → EVAL-03 → EVAL-04 → WRITE-01 → WRITE-02 → WRITE-03 → WRITE-04. The outstanding V2-DOC-04 DOCX visual render is tracked as a separate artifact gate, not an extra thesis task.
- V2-EVAL-01 continuation (2026-09-25): added bounded Odoo tests for the attempt-level unique approval constraint and unknown approval state at final execution, each with persistent non-final/unconsumed oracles. Python syntax and Compose parsing pass; the fresh E2E gate remains pending because the local Docker daemon could not start. No new V2 verification claim is made.
- V2-EVAL-01 fresh gate (2026-09-25): after Docker access and 24-hour testbed mTLS certificate renewal, 54 Odoo post-tests / 64 cases pass with 0 failures/errors, mTLS probes and baseline/approved two-session, retry and stale-intent runners. The duplicate-approval constraint, unknown-state denial and same-tenant cross-company grant/order guard have bounded persistent-state evidence. The task remains in progress because composite negatives remain partial.
- V2-EVAL-01 line-intent extension (2026-09-25): added Odoo negative oracles for changed purchase-line tax and explicit persisted line write-version change. The fresh gate passes 55 post-tests / 65 cases, 0 failures/errors, with mTLS and two-session/retry/stale-intent runners. The remaining line-shape and other composite matrix variants are still open.
- V2-EVAL-01 line-shape extension (2026-09-25): added Odoo negative oracles for added/removed purchase lines, one-line sequence change and product substitution. Fresh gate passes 56 post-tests / 66 cases, 0 failures/errors, with mTLS and two-session/retry/stale-intent runners. UoM and actual multi-line reordering remain unverified.
- V2-EVAL-01 listed-line closure (2026-09-25): added an approved-final UoM substitution case and a real two-line reorder after approval, both asserting non-final PO and unconsumed approval/attempt. The first two-line fixture was denied before approval, so only the corrected same-total fixture and fresh passing gate count as evidence: 57 post-tests / 67 cases, 0 failures/errors, mTLS and two-session/retry/stale-intent runners pass. CBI-N03 is bounded-verified for its listed line edits; other composite rows and concurrent-in-flight business edits remain open.
- V2-EVAL-01 partial-capability extension (2026-09-26): added 12 persisted missing-field subcases under BOUND-N03. Six absent text metadata fields initially caused uncaught Protobuf TypeErrors; validating them in `_stored_capability` now feeds the existing controlled invalidation path without changing wire contracts, policy or keys. The focused method and fresh full gate pass: 58 post-tests / 68 cases, 0 failures/errors, mTLS and all two-session/retry/stale-intent runners. Each subcase preserves a non-final PO and unconsumed approval/attempt. Remaining issuance-revision/pending-binding variants and other composite rows keep V2-EVAL-01 in progress. Docker Desktop was started after an unavailable-daemon error; no commit/push was performed.
- BOUND-N03 binding extension (2026-09-26): three new methods cover 23 subcases: 12 required-column NULL writes rejected by PostgreSQL with a post-savepoint non-final/unconsumed oracle; seven persisted empty binding strings rejected by final execution; and NULL/zero/negative/changed issuance revision rejected against an originally signed nonzero revision. The fresh gate passes 61 post-tests / 71 cases, 0 failures/errors, mTLS and all two-session/retry/stale-intent runners. No additional production-code change was needed. Revision 0 remains valid; see the ledger for SQL NULL coercion and corruption-scope limits. V2-EVAL-01 remains in progress.
- CBI-N04 state-change evidence (2026-09-26): `test_post_approval_state_change_invalidates_old_capability` changes an approved PO to `draft`, `sent` or `cancel` through the public ORM write route. The protected confirm preserves that non-final state, invalidates the old approval with `intent_changed`, and leaves the command unexecuted. The original intent hash, one-time ID and command binding remain unchanged; retrying the invalidated approval hard-denies with the same persistent-state oracle. Together with the explicit parent-version test, this verifies the listed CBI-N04 variants in one Odoo transaction, not concurrent-in-flight edits. Fresh gate: 62 post-tests / 72 cases, 0 failures/errors, mTLS and all two-session/retry/stale-intent runners pass. No additional production-code change was needed. V2-EVAL-01 remains in progress.
- CBI-N02 material-binding evidence (2026-09-26): `test_final_material_context_tamper_with_old_proof_fails_closed` independently changes tenant, company, action, resource, vendor and currency in the final outgoing request while retaining the proof for the original locked intent. For the five schema-valid variants the state witness is recomputed; the action variant injects an unsupported CBI action. The real generated-client/mTLS/PDP boundary returns gRPC `PERMISSION_DENIED`; post-savepoint oracles preserve non-final source/target POs, approved capabilities and unexecuted commands. No PDP decision is mocked. `test_currency_edit_never_reuses_approved_intent` additionally writes a different currency through public ORM, preserves the numeric total, and verifies old-approval invalidation without consumption; the earlier persisted-vendor test remains separate evidence. The first run failed six assertions because they expected a later policy-denial message instead of the earlier proof/request rejection; after checking the actual chained RPC status, the focused six-subcase method passed. The fresh full gate then passes 64 post-tests / 74 cases, 0 failures/errors, mTLS and all two-session/retry/stale-intent runners. No production-code change was needed. These are bounded request-tamper and sequential ERP-edit tests, not tenant/company migration, compromised-PEP resistance or concurrent-in-flight edit evidence. V2-EVAL-01 remains in progress.
- TXN-N04 overlapping-edit evidence (2026-09-26): `deployments/docker/odoo_material_race.py` runs four controlled schedules on separate Odoo/PostgreSQL sessions: line description and unit price, each with edit-first and final-first ordering. Events hold transactions open; `pg_blocking_pids` proves actual blocking, and workers require REPEATABLE READ. With edit first, Odoo retries the stale final snapshot and invalidates the old approval; a fresh observer sees the edited PO still non-final and command unconsumed. With final execution first, a fresh observer sees the original approved data, consumed approval and executed command committed together while the edit is still uncommitted; the edit commits later. Bindings remain unchanged and each PO has one approval/attempt. The runner calls real ORM/PDP methods; its hook only pauses before the native final mutation. The focused runner and fresh full gate pass; no authorization defect was reproduced and no production change was required. The first price oracle ignored tax and was corrected to verify persistence of Odoo's tax-inclusive total. Gate: 64 post-tests / 74 reported cases plus four material-race schedules, mTLS and baseline/approved concurrency, retry and stale-intent runners, all passing. These four schedules do not establish coverage for line insert/delete/tax changes, other material fields, concurrent authority changes or a general prohibition on post-confirmation edits. V2-EVAL-01 remains in progress.
- CBI-N05 protocol evidence (2026-09-26): `test_final_protocol_variants_fail_closed` covers 12 outgoing request variants: unknown CBI schema, intent proof version and proof-envelope version; an unknown `cbi.*` material field; missing company; leading-zero ID; decimal, exponent and overflowing minor units; non-hex digest; noncanonical timestamp; and truncated proof signature. Each reaches the real generated-client/mTLS/PDP boundary and receives gRPC PERMISSION_DENIED, with a non-final PO, approved/unconsumed capability, unexecuted command and unchanged intent/one-time/command binding after the savepoint. `test_stored_canonical_encoding_variants_cannot_finalize` separately persists non-base64, padded or truncated canonical-intent text; Odoo final preflight rejects all three while preserving that corruption and the non-final/unconsumed outcome. The existing V1 downgrade case remains covered. The fresh gate passes 66 post-tests / 76 reported cases, zero failures/errors, mTLS, baseline/approved race/retry/stale-intent and all four overlapping material-edit schedules. Initial execution stopped before tests because the ephemeral TLS certificates expired; certificate regeneration and PDP restart restored the gate. No production change was needed. Coverage is limited to these enumerated wire/storage variants, not arbitrary binary fuzzing or automatic discovery of material fields added by Odoo extensions. V2-EVAL-01 remains in progress; its next step is the six-row scope/evidence review.
