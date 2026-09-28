# V2-EVAL-04 — Claim-to-evidence reconciliation and threats to validity

Status: **VERIFIED V2 — BOUNDED CLAIM ANALYSIS (2026-09-28)**.

Authority: current source/protocol/tests, then the
[current-state audit](../CURRENT_STATE_AUDIT.md). This ledger interprets accepted
evidence; it adds no runtime result and does not replace the audit.

## Verdict

The evidence supports a bounded result: for the tested Odoo 17
`purchase.order` confirmation path, the implementation enforces one-hop delegated
authority, exact transaction intent, required exact-action approval and final
database transaction checks under the documented trusted-writer, trigger and clock
assumptions. It also demonstrates the mechanism against two explicit ablations and
records separated feasibility measurements.

The evidence does not establish general ERP security, SAP compatibility,
production readiness, legal compliance, HMAC non-repudiation, prompt-injection
prevention, instant/global revocation, distributed atomicity, exactly-once external
effects, causal approval overhead or superiority over policy engines in general.

## Claim-to-evidence map

| Claim permitted in the thesis | Direct evidence | Required wording boundary |
|---|---|---|
| **C1 — Delegated identity and authority:** the protected path binds a trusted agent caller to one active same-tenant human delegation with scope, validity and constraints. | Current source/tests summarized by [EVAL-01](V2_EVAL_01_CASE_LEDGER_2026_09_24.md); `AUTH-P01`, `AUTH-N01`, `AUTH-N02` in the [matrix](../EVALUATION_MATRIX.md). | One hop and the configured Odoo/PDP identity path only. Frontend identity handoff, arbitrary identity providers and multi-agent delegation are excluded. |
| **C2 — Exact intent and state binding:** canonical intent reconstructed from locked ERP records detects the enumerated request substitutions, material edits, stale states and protocol/storage faults. | `CBI-P01`, `CBI-N01`–`CBI-N05`, `TXN-N04`; shared Go/Python vectors and persistent-state oracles in EVAL-01. | Claim only the listed fields, entry points and edit schedules. No arbitrary Odoo extension, compromised PEP, arbitrary SQL or universal field-discovery guarantee. |
| **C3 — Exact-action approval and SoD:** AC v1 binds one authorized independent human, intent/state, expiry and one-time ID; listed substitution, replay, expiry, key-confusion and authority cases fail closed. | `APP-P01`, `APP-P02`, `APP-N01`–`APP-N05`; issuance/final-execution evidence in EVAL-01. | HMAC gives integrity to configured key holders, not legal signature or non-repudiation. No general approval supersession protocol. |
| **C4 — Commit-time enforcement:** within the tested Odoo/PostgreSQL transaction, final execution revalidates current protected state/authority and atomically commits the PO mutation with command and required approval consumption. | `TXN-P01`, `TXN-N01`–`TXN-N04`, configured policy/revocation fence and deferred-expiry schedules in EVAL-01; current audit. | All in-scope writers must use the configured ERP fence; local triggers and database UTC clock are trusted. This is not distributed atomicity or validity at WAL/network-delivery time. |
| **C5 — Retry and fail-closed behavior:** the bounded route handles lost-response retry, duplicate execution, injected rollback and listed PDP/capability outages without an unauthorized committed PO mutation. | `TXN-P02`, `BOUND-N01`–`BOUND-N03`, fresh observer checks in EVAL-01. | At-most-once scoped database effect only. External side effects, crash recovery, broad availability and automatic publication recovery remain unverified. |
| **C6 — Selected baseline distinction:** in 11 fixed scenarios, variant C preserves valid direct/approved/retry behavior and rejects selected stale/revoked/substitution cases that the chosen A/B ablations permit. | [EVAL-02](V2_EVAL_02_COMPARISON_2026_09_27.md): 11 scenarios × 3 variants, 33 committed/fresh-observer outcomes. | A/B bundle different controls and are experiment-only adapters. Do not infer universal superiority, compare with OPA/Cedar, or attribute each difference to one isolated mechanism. |
| **C7 — Bounded feasibility measurements:** evaluator, proof/capability, warm RPC, locked CBI reconstruction and Odoo mutation/commit distributions were recorded separately with raw samples and environment fingerprints. | [EVAL-03](V2_EVAL_03_MEASUREMENT_2026_09_27.md), accepted Go/Odoo JSON and combined summary linked there. | Go micro-workloads and Odoo spans are different boundaries. Nested spans overlap. No subtraction of timer percentiles, causal approval overhead, production SLA, concurrent-load or speedup claim. |
| **C8 — SAP relevance:** the invariant identifies questions that a future SAP implementation would need to answer. | Architectural applicability table in [scope alignment](../../thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md). | Discussion only; no SAP runtime, compatibility, equivalent control or performance result. |
| **C9 — AI-agent relevance:** an agent is modeled as an untrusted non-human software principal that proposes tool actions under controlled delegation. | Threat model and protected Odoo tool boundary; deterministic authorization negatives in EVAL-01/02. | No LLM quality, prompt-injection experiment, model training, orchestration or autonomous-agent benchmark. |

## RQ conclusions

| RQ | Bounded answer | Confidence and remaining blind spot |
|---|---|---|
| **RQ1 — Who may act for whom?** | The tested route accepts only the authenticated agent acting through a valid one-hop same-tenant grant and current configured authority. | High for enumerated Odoo/PDP cases; external identity lifecycle and unconfigured writers are outside the boundary. |
| **RQ2 — Is authority bound to this exact current transaction?** | CBI/proof plus locked reconstruction and final checks bind the listed material fields, state witness and command to the protected confirmation. | High for listed fields/schedules; no proof of completeness for arbitrary extensions or compromised trusted code. |
| **RQ3 — Is required human approval valid and single-use?** | AC v1 and database lifecycle checks bind an independent current approver to one intent/state and consume approval with the mutation. | High inside the tested database transaction; no legal non-repudiation or external-effect exactly-once. |
| **RQ4 — What changes relative to simpler controls, and what does it cost?** | Selected A/B/C outcomes expose controls supplied by transaction binding, while EVAL-03 reports separate boundary distributions and errors. | Moderate: selected ablations are not universal baselines, workloads differ, no concurrent/production load, and causal marginal overhead is not isolated. |

## Threats to validity

### Construct validity

- The study models an AI agent as a software principal; it does not run an LLM or
  measure prompt injection. Results concern authorization after a tool proposal.
- One PO confirmation represents a high-impact action but does not represent the
  whole Procure-to-Pay process or all ERP mutations.
- EVAL-02 variants intentionally bundle controls. Their outcomes show behavior of
  those concrete ablations, not the independent causal contribution of each field,
  hash, policy check or approval step.
- Odoo Activity is treated as notification and AC v1 as approval evidence. This is
  an explicit design choice, not a claim about all ERP approval workflows.

### Internal validity

- The trusted runner bypasses a frontend and real agent tool stack. Odoo/PDP mTLS,
  JWT and generated-client boundaries are exercised, but browser/session identity
  propagation is not.
- Source was a recorded dirty worktree at commit `304c1f5`, not a signed release.
  Raw files hash relevant mounted sources, but full build provenance is absent.
- Correctness depends on every configured policy/revocation writer using the same
  ERP fence, intact local-authority triggers and the PostgreSQL UTC clock.
- Warm-up excludes cold start from accepted latency distributions. Measurements are
  sequential single-writer runs; scheduling noise and one AC maximum outlier remain
  in raw data rather than being removed.
- Instrumented Odoo spans are inclusive and nested. The approved and direct routes
  differ in amount and workflow state, so their total difference is not a controlled
  estimate of approval overhead.

### External validity

- Runtime evidence covers Odoo 17 Community, PostgreSQL 15 and one
  `purchase.order` confirmation path with finite fixtures and enumerated races.
- No SAP runtime, other ERP, multi-tenant scale, concurrent workload, arbitrary
  custom module, payment/external side effect or geographically distributed system
  was evaluated.
- Financial EVAL-02 fixtures use integral major-unit values even though exact
  minor-unit reconstruction has separate tests. Broad fractional policy behavior is
  not established by the comparison.

### Conclusion and reproducibility validity

- EVAL-01 IDs, post-tests, runner schedules and EVAL-02 outcomes are different
  units; they must not be summed into one success count.
- EVAL-03 percentiles use one recorded environment and linear interpolation. QPC
  timer/predicate overhead and 100 ns quantization are included. Micro-latency does
  not establish end-to-end throughput or production capacity.
- Failed diagnostic artifacts are retained and excluded from accepted outcomes.
  EVAL-02 exposed a real deferred line-flush defect; EVAL-03 exposed invalid Windows
  timer assumptions and a too-strict empty-control oracle. These repairs strengthen
  the specific regression boundary but do not prove absence of other defects.
- Publication recovery is manual and unverified end to end; coarse authority-epoch
  contention, crash recovery and ambiguous commit timeout costs remain unmeasured.

## Evidence that would change the verdict

A broader verdict requires, at minimum: signed/reproducible release evidence; a real
frontend and agent tool path; independent workloads across additional ERP actions;
tests for extension fields and writer bypass; controlled factorial baselines;
concurrent/cold-start/load measurements; recovery drills; and a separate SAP runtime
implementation before any SAP claim.

## Writing rule

Chapters 3–5 may state C1–C9 only with the boundary in the same paragraph or table.
Use the exact denominators from their source reports. Historical V1 speedup and fixed
nanosecond claims remain retired. Conflicts are resolved by source/tests and the
current-state audit, not by this synthesis.
