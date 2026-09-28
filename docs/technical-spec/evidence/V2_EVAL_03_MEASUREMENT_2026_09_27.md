# V2-EVAL-03 — Boundary-separated measurement contract and results

Status: **VERIFIED V2 — BOUNDED MEASUREMENTS (2026-09-27)**.
Date: 2026-09-27. EVAL-03 acceptance closed after the corrected Go collection below.
Authority: [active checkpoint](../../../ACTIVE_TASK.md), [matrix](../EVALUATION_MATRIX.md),
[accepted EVAL-02 boundary/limits](V2_EVAL_02_COMPARISON_2026_09_27.md).

## Scope and measurement contract

Only the existing protected Odoo 17 PO workflow is measured. Production authorization,
native workflow, locks, proof verification and deferred commit checks are unchanged.
The isolated `odoo_eval_abc` DB is reused with new fixtures; no DB reset is required.
The runner does not invoke the A/B ablation adapters. Setup uses the same trusted
creator/service/approver fixtures, not an HTTP login or LLM tool stack.

All times are integer nanoseconds from monotonic clocks. Instrumentation calls the
original function and propagates exceptions. Wrappers are restored on exit. Start/end
mean immediately around the named call, unless the total boundary says otherwise.

| Raw boundary | Included | Excluded / interpretation |
|---|---|---|
| `evaluator_single_permit` | Go `CheckPermission` with one compiled permit, amount 1000, result predicate | Compilation, fixture setup, JWT, proof, DB, RPC. A micro-workload, not the five-policy ERP deployment. |
| `proof_v2_verify` | Go `VerifyProofV2` validation, real-clock validity, CBI hashing and HMAC verification, result predicate | Signing, key/fixture setup, revocation store, DB, RPC. Golden CBI input, not an ERP transaction. |
| `capability_v1_verify` | Go `ApprovalCapabilityManager.Verify`, real-clock validity and result predicate | Issuance/random generation, current approver policy, DB consumption, RPC. AC is derived from the golden CBI. |
| `timer_control` | Go empty successful operation plus timer/predicate path; Windows collector now uses QPC | Not subtracted; timer overhead/quantization can dominate tiny operations. |
| `locked_cbi` | Python `build_purchase_order_intent`: flush, locked PO/line reads, identity checks, exact money, line digest, state witness | PDP, proof signing, final mutation and commit. Records each invocation, including repeated reconstruction within a transaction. |
| `python_proof_sign` | `sign_delegation_proof_v2` API call on reconstructed input | ERP reconstruction, key-ring loading, RPC. Extra diagnostic, not Go verifier latency. |
| `rpc_check_access` | `SafePDPClient.check_access`: request construction, generated Protobuf, warm mTLS transport, server processing/fence work, response conversion/checks | JWT issuance, initial channel/handshake warm-up. NOT pure network or pure evaluator cost. Approved-route calls include approver and agent checks, pooled only within that route. |
| `rpc_verify_capability` | Full Python capability-verification RPC including request/response conversion | Separate from standalone Go AC verification; not its additive overhead. |
| `native_button_approve` | Native purchase addon `PurchaseOrder.button_approve` and its synchronous descendants, including installed ORM guards | Outer authorization/approval consumption, earlier `button_confirm` preparation, deferred flush and commit. A mutation-call component, NOT all ERP mutation cost. |
| `protected_button_confirm` | Complete protected `order.button_confirm` call, authorization, binding, command/AC state writes, native mutation | Final `cr.commit`; any pending ORM work is completed there. |
| `commit` | `cr.commit` through return: pending flush/deferred constraints and PostgreSQL commit processing | Earlier method body, later fresh observer; not an external-effect or durability SLA. |
| `confirm_through_commit` | Protected confirm plus commit, inclusive of nested timing wrappers | Cursor/environment creation, PO/grant setup, approval preparation/wait, observer, retry. No automatic retries are used. |

The Odoo collector retains span IDs/parent IDs. All spans are **inclusive**. Do not
sum them, subtract Go microseconds from ERP timings, or call the remainder network
overhead. Python's 1,000 empty-wrapper inner durations are a timer diagnostic, not
the complete wrapper/bookkeeping overhead; no calibration is subtracted.

## Workload, denominators and interpretation

- Go: default 100 warm-up + 1,000 recorded invocations **per boundary**, one worker,
  rotating boundary order. Native host Go environment is recorded separately from Docker.
  Fixed valid inputs, real clock, signing before timing; no `-race` instrumentation.
- Odoo: default 20 warm-up + 200 samples **per route**, direct 1000 USD and approved
  2500 USD, one PO line, sequential single writer. Alternate route order each iteration.
  Every PO/grant is newly committed. Approved setup commits pending and independent
  human issuance in separate transactions before the measured final execution.
- Both routes use normal protected calls as a non-superuser. After commit, a fresh
  cursor must see `purchase`, exactly one executed command and, for approved, exactly
  one consumed capability. Material inputs must be unchanged. Missing expected hooks
  fail the run. No setup/approval/observer duration is part of final latency.
- Warm-up rows are retained, labeled and excluded from summaries. Each event's
  invocation is a sample, not one event per transaction; report those counts. The
  outer total has one sample per transaction. Multiple CBI/RPC invocations must not
  be reported as independent transactions.
- Summary uses linear interpolation `(n-1)*p` for p50/p95/p99 and observed maximum.
  It reports attempts, unsuccessful count, and separate successful/unsuccessful
  distributions. A passing inner call in a failed transaction is excluded from the
  successful distribution. Empty sets have null percentiles, not zero latency.
- Odoo stops at the first failed preparation/transaction/oracle, retains its raw row
  and failed-run metadata. Planned counts and actual rows expose an incomplete run;
  setup failure may have no timing events. Go records non-passing measured results
  and returns a failing test. Setup/build failures are not latency observations.
- Source hashes, Git revision, versions, OS/CPU count and timer metadata accompany
  samples; PostgreSQL version/isolation/synchronous-commit are recorded by Odoo.
  Fingerprints exclude credentials and capability/proof payloads. Outputs refuse
  overwrite. Capture Docker image IDs and host/background-load notes with final results.
- These are warm, uncontended, instrumented samples, not throughput, concurrency,
  cold-start, production SLO, robust tail estimation or general ERP overhead evidence.
  Host Go and Docker RPC workloads/platforms differ. With 200 transactions per route,
  p99 is descriptive near the sample tail; no confidence interval or speedup is claimed.

## Preparation validation (Sol group, completed)

- Four focused Python tests pass: percentile/empty/invalid input; transparent nested
  wrappers and exception propagation; warm-up/failure/route separation; artifact
  count/failure/fingerprint checks. Five Python files parse; Go source is gofmt-clean.
- [Go smoke raw](V2_EVAL_03_GO_SMOKE_20260927T1458279269811Z.json): PASS, one warm-up
  and three samples per four boundaries, 16 rows. Real API calls; no final latency claim.
- [Odoo smoke raw](V2_EVAL_03_ODOO_20260927T145507072127Z.json): PASS, one warm-up
  and two samples per route, six committed transactions/fresh-observer outcomes.
- [Recomputed smoke summaries](V2_EVAL_03_SMOKE_SUMMARY_20260927T1458Z.json): both raw
  artifacts pass schema/count validation; stored Odoo summary matches recomputation.
  `kind=smoke` remains visible and must not be promoted to full evidence.
- Preparation diagnostics fixed: Go cache required sandbox approval; same-package
  engine import caused a test dependency cycle (resolved with external test package);
  first AC fixture ID was 17 rather than 16 bytes; percentile unit assertion needed
  floating-point tolerance. No production-code defect or change resulted.
- The Odoo smoke retains the source snapshot at execution; subsequent changes only
  repaired Go/test helpers. No protected runtime input changed. EVAL-02's existing
  correctness gate is reused; no redundant full ERP suite or document export rerun.

## Historical first-collection procedure — already executed

The commands in this section produced the first full artifacts below. They are
retained for reproduction and superseded by the accepted final collection later in
this report. The Odoo collection was retained for the clock repair and was not rerun.

Read only `ACTIVE_TASK.md`, this section and relevant raw outputs. Run the following
once, sequentially, from the repository root; avoid concurrent benchmark processes.
Do not rebuild the already healthy testbed, rerun A/B/C or reset a database. If the
testbed/source/credentials differ or any command fails, preserve diagnostics and
stop for a **Sol** handoff before diagnosis or code changes. Do not run all commands
blindly after a failed exit code. Docker/Go may need the existing sandbox approval.

```powershell
$evalPython = 'C:/Users/Admin/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe'
$env:PDP_GIT_COMMIT = git rev-parse --short HEAD
$env:PDP_EVAL03_SMOKE = '0'
$env:PDP_EVAL03_SAMPLES = '1000'
$env:PDP_EVAL03_WARMUP = '100'
$evalStamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssfffffffZ')
$env:PDP_EVAL03_GO_OUTPUT = Join-Path (Get-Location) "docs/technical-spec/evidence/V2_EVAL_03_GO_$evalStamp.json"
go test ./internal/security -run '^TestEval03Latency$' -count=1 -v
```

Then, only if Go passed:

```powershell
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark ps --format json
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark images --format json
docker compose -f docker-compose.testbed.yml -f docker-compose.eval.yml -p pdp-eval-abc --profile benchmark run --rm --no-deps --entrypoint python3 testbed-odoo-benchmark /opt/pdp-eval/odoo_eval_latency.py --samples 200 --warmup 20
```

Odoo prints the exact timestamped `EVAL03_RESULT` filename under `/results`, mounted
at `docs/technical-spec/evidence`. Pass that **actual new** file and the actual Go
output to `deployments/docker/summarize-eval-latency.py`, using the bundled Python:

```text
<python> deployments/docker/summarize-eval-latency.py <new-Go-JSON> <new-Odoo-JSON> --output <new-summary-JSON>
```

Do not pick files by an unrestricted "latest JSON" glob or include smoke artifacts.
Check `kind=measurement`, PASS, Go 4,400 rows and Odoo 440 rows, warm-up excluded in
summary, exact new filenames, per-boundary denominators, source/image/environment
metadata, and observer outcomes. These were the original collection instructions;
their completion and accepted artifacts are recorded in the final section below.
The first Go artifact was later rejected after clock diagnosis, and the subsequent
QPC attempt was rejected only for two unresolved empty-control samples. Neither is
included in accepted metrics.

## First full-collection outputs — Odoo retained, original Go rejected

Both fixed collection commands exited 0 on 2026-09-27. Go retained 4,400 rows
(100 warm-up + 1,000 samples per boundary); Odoo retained 440 rows (20 warm-up +
200 measured transactions per route). Every Odoo protected mutation committed, a
fresh observer saw exactly one executed command, approved transactions consumed one
capability, and all listed boundary spans had zero call errors. Percentiles below
use the script's linear `(n-1)*p` interpolation and are in milliseconds.

| Odoo boundary | Route | Event samples | p50 | p95 | p99 | max | Errors |
|---|---:|---:|---:|---:|---:|---:|---:|
| `confirm_through_commit` | direct | 200 | 70.521 | 139.866 | 152.247 | 174.005 | 0/200 |
| `confirm_through_commit` | approved | 200 | 75.642 | 144.614 | 174.623 | 179.492 | 0/200 |
| `protected_button_confirm` | direct | 200 | 47.117 | 96.170 | 116.291 | 133.662 | 0/200 |
| `protected_button_confirm` | approved | 200 | 47.687 | 92.375 | 120.325 | 132.999 | 0/200 |
| `commit` | direct | 200 | 22.002 | 48.144 | 62.445 | 66.424 | 0/200 |
| `commit` | approved | 200 | 26.989 | 58.949 | 67.893 | 90.160 | 0/200 |
| `locked_cbi` | direct | 200 | 4.360 | 11.716 | 13.279 | 14.336 | 0/200 |
| `locked_cbi` | approved | 600 | 3.970 | 10.284 | 15.037 | 19.404 | 0/600 |
| `rpc_check_access` | direct | 200 | 1.964 | 5.329 | 6.242 | 6.801 | 0/200 |
| `rpc_check_access` | approved | 400 | 1.345 | 3.403 | 5.359 | 6.774 | 0/400 |
| `rpc_verify_capability` | approved | 200 | 1.061 | 2.632 | 3.305 | 4.490 | 0/200 |
| `native_button_approve` | direct | 200 | 0.536 | 1.607 | 2.038 | 3.487 | 0/200 |
| `native_button_approve` | approved | 200 | 0.541 | 1.581 | 2.349 | 2.635 | 0/200 |

The approved route performs three locked CBI reconstructions and two policy RPCs
per measured transaction in this implementation; event counts therefore differ
from transaction counts. Its policy RPC distribution contains both approval-authority
and agent checks. Direct and approved workloads differ in value and approval state;
their totals are not a controlled overhead comparison. Inclusive spans overlap and
must not be added. The native-call span excludes pre-transition work and commit.

The Go artifact **does not provide usable micro-latency distributions**. In 1,000
samples, the evaluator and empty-timer control both report exactly 0 ns every time;
proof verification reports 0 ns in 980/1,000 samples and capability verification
in 991/1,000. Nonzero maxima reach 1.3468 ms amid those zeros. This indicates timer
resolution/measurement validity must be diagnosed before any Go percentile is
published. The JSON and computed summary remain preserved as diagnostic artifacts;
do not cite their Go percentiles or infer engine speed. The corrective work and
its focused validation are recorded below.

Artifacts:

- [Go raw run](V2_EVAL_03_GO_20260927T1504251123581Z.json), SHA-256
  `68f8ff50b4af0be0ace2fd127c9d205f4faf1e2ddd502d93c5f24fde52a29aa8` — diagnostic,
  measurements flagged unusable as above.
- [Odoo raw run](V2_EVAL_03_ODOO_20260927T150456575495Z.json), SHA-256
  `a67b664b721d24976e84b166e50a683f10acea8100037104d5fe914a77b6706d` — full
  warm sequential sample set and fresh-session persistence oracles.
- [Arithmetic/count-validation output](V2_EVAL_03_SUMMARY_20260927T1507191913661Z.json).
  It faithfully computes Go zero-valued samples; its PASS is schema/count validation,
  not an assertion that the Go timing instrument has adequate resolution. Summary
  SHA-256: `768855FD178E70EF8AD8CC11DC9B1A3A8A2D51197924615AD24DA302D5711388`.
- Odoo: 17.0-20260908, Python 3.10.12, PostgreSQL 15.19, Linux/WSL2, 20 visible CPUs,
  REPEATABLE READ, synchronous_commit=on, Git `304c1f5` plus dirty worktree. PDP image
  `sha256:fcf2b20b7f1bfdc6486609d62313aec76627bb0f08c90e9f1eb95f70e3c344ca`, PostgreSQL
  image `sha256:fe0737ba566a2c5b2a28f34433c0a423261900ec17b9bf7ad115e1aae7e57f1b`,
  Odoo client image `sha256:bb8c9b9c05b8c4f3523204bf879be3baadabe4fe58814ecea96a51add7ba97c3`.
  Windows host CPU model collection was denied, so it is not asserted. Raw source hashes
  are retained in each run; measured CBI/client source hashes match the smoke baseline.
- Go: go1.26.4 windows/amd64, 20 visible CPUs, Git `304c1f5` plus dirty worktree.
  This in-process micro-workload and its current raw timings are not evidence for an
  ERP transaction. No cross-platform or speedup comparison is made.

## Sol review and clock repair, 2026-09-27

Root cause: the installed Go 1.26.4 Windows/amd64 source reads `_INTERRUPT_TIME`
for both `time.now` (`runtime/time_windows_amd64.s:11`) and `runtime.nanotime1`
(`runtime/sys_windows_amd64.s:200`). Nanosecond units do not establish nanosecond
resolution. The original per-call collector used that coarse source, consistent
with its zero-valued readings. This conclusion concerns the installed runtime;
it is not a claim about every Go version/platform.

The repository already uses `QueryPerformanceCounter` in
`tests/perf_clock_windows_test.go`. The EVAL-03 collector now follows that pattern
with test-only, platform-specific clock files under `internal/security`. QPC frequency
and nominal tick are recorded. On this host: 10,000,000 Hz / 100 ns ticks. Security
expiry clocks are untouched. Clock syscall/dispatch and predicate overhead remains
included; show the timer control alongside APIs and do not subtract percentiles or
present the small evaluator span as intrinsic engine cost.

The collector marks nonpositive elapsed times as failure and preserves the artifact.
The summarizer independently rejects unresolved API timings even if the input
declares PASS. Timer-control events require a separate rule because an empty function
can finish within a single QPC tick.

Focused validation:

- Five Python tests pass, including unresolved-timer rejection.
- [QPC smoke](V2_EVAL_03_GO_QPC_SMOKE_20260927T1518497245881Z.json): 5 warm-up +
  20 samples per boundary, 100 rows, all positive and API results valid. Measured
  minimum durations were 100 ns control, 300 ns evaluator, 7,300 ns proof and
  2,700 ns AC; these smoke values are diagnostic, not final distributions.
- [Smoke/Odoo summary validation](V2_EVAL_03_QPC_SMOKE_SUMMARY_20260927T1518Z.json)
  accepts the corrected smoke and existing full Odoo artifact. A separate invocation
  on the old Go artifact exits 1 with `Unresolved/invalid timer sample` as expected;
  no accepted replacement file is emitted for it.
- Reviewed all 440 retained Odoo rows: commit returned, persisted final state and
  executed command, approved AC consumption, positive spans, unique resolvable
  parent IDs with durations containing children; no findings in those checks.
  The source wrappers call originals, setup/approval/observer are excluded from the
  measured final total, and the two protected routes have their documented meanings.
- The retained Odoo environment reports Linux `clock_gettime(CLOCK_MONOTONIC)`,
  nominal resolution 1 ns, unlike the original Go host clock. The full Odoo sample
  set can be retained for these bounded instrumented API measurements. No new ERP
  security claim, concurrent-load measurement or full Odoo rerun is needed.

## Corrected full run diagnostic — do not accept

The QPC full run retained [4,400 rows](V2_EVAL_03_GO_QPC_20260927T1524281846618Z.json),
SHA-256 `5EF9C9ACA5C00405ECA9972645C799496984CFCBC3657F0667A83D327A3A6775`, but exited
1. Exactly two measured `timer_control` rows (sample indices 472 and 596) read 0 ns;
each was marked `timer_not_resolved`, causing the overall FAIL. There are 3,000 measured
API calls and 1,000 measured controls, not 4,000 API calls. All API calls and all
400 warm-ups (300 API + 100 control) have positive durations and valid results. QPC is
10 MHz/100 ns per tick, so consecutive reads around an empty function can return the
same tick. This is an overly strict timer-control oracle, not an API failure.

The current summarizer rejects the whole FAIL artifact. No summary was emitted. Keep
this diagnostic file; do not include it as accepted performance evidence.

## Timer-control repair and final Go collection

The Go collector now permits zero only for the empty `timer_control` boundary;
negative elapsed times and failed operations still fail the run. The summarizer
permits the same exception only on `go_micro` rows with the per-boundary Go plan;
all API and Odoo times remain positive integers. Controls stay in the summary with
their true zero values, counts and quantization caveat. No stored FAIL artifact is
rewritten or reclassified.

Validation before the full run: six Python tests, including complete synthetic
zero-control acceptance, retained zero count/percentile, rejection of zero API,
invalid control values, wrong-route control and failed-row status; the
[100-row Go smoke](V2_EVAL_03_CONTROL_FIX_SMOKE_20260927T1532087312311Z.json) and
[smoke/Odoo summary](V2_EVAL_03_CONTROL_FIX_SUMMARY_20260927T1532087312311Z.json)
passed. The full corrected Go run then passed once with 100 warm-ups and 1,000
measured samples per boundary. Its 4,400 rows comprise 3,000 measured API calls,
1,000 measured controls, 300 API warm-ups and 100 control warm-ups. All operation
results were valid; all API durations were positive. The 440-row full Odoo artifact
was reused unchanged and remains independently observer-checked.

Accepted [Go raw output](V2_EVAL_03_GO_QPC_20260927T1538104523574Z.json),
SHA-256 `7812cf7a50fe830cda64ea23224cf8e753ec09b08978f12e58513e11ce86974b`;
accepted [Odoo raw output](V2_EVAL_03_ODOO_20260927T150456575495Z.json),
SHA-256 `a67b664b721d24976e84b166e50a683f10acea8100037104d5fe914a77b6706d`;
and [recomputed combined summary](V2_EVAL_03_SUMMARY_20260927T1538104523574Z.json),
SHA-256 `80e50a15d36309b6fbbb5a9891dd283b5b4ce7aff48c240e713bdedc1df7c79a`.
The summary validator accepted exactly these two `kind=measurement`, PASS runs.

Go results (1,000 successful measured calls per row; warm-up excluded):

| Boundary | Errors | p50 (µs) | p95 (µs) | p99 (µs) | Max (µs) |
|---|---:|---:|---:|---:|---:|
| `timer_control` | 0 | 0.200 | 0.300 | 0.401 | 2.100 |
| `evaluator_single_permit` | 0 | 0.600 | 1.500 | 5.000 | 228.700 |
| `proof_v2_verify` | 0 | 11.000 | 22.820 | 42.413 | 179.200 |
| `capability_v1_verify` | 0 | 3.900 | 10.100 | 21.615 | 3,194.000 |

The Go clock is QPC at 10 MHz (nominal 100 ns ticks). Clock syscall, dispatch and
result-predicate work remain included; the control row shows the instrument path and
quantization. Do not subtract the control distribution, add percentiles, or treat
the evaluator figure as intrinsic production-engine latency. The Go calls are
in-memory micro-workloads without DB/RPC; the Odoo routes are separate bounded
single-writer workflows. Direct/approved Odoo workloads differ in amount and
approval state, so their totals do not establish causal approval overhead. No
concurrent-load, general ERP, SAP, production SLA or speedup claim follows.

EVAL-03 is closed for these bounded measurements. Failed historical Go runs remain
unchanged and excluded from accepted results. The next task is V2-EVAL-04 claim-to-
evidence reconciliation and threats-to-validity review; pause here for the requested
Sol model handoff. The separate DOCX visual gate remains open. No commit or push was
performed.
