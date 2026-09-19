# Thesis Chapter Mapping and Evidence Plan

> **Status:** 2026-09-19. Chapters 3–5 below are a repository-Markdown draft from the frozen `e243db5` evidence. They are not a production-readiness declaration. Read [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md) and [`THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`](../thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) first.

## 1. Research-question evidence matrix

| RQ | Thesis claim to establish | Available evidence | Evidence still required |
|---|---|---|---|
| RQ1: unified subject and delegation | A policy request can distinguish an authenticated actor, AI-agent context and constrained 1-hop delegation. | Tenant-bound JWT, versioned full-tuple proof, repository-owned Odoo mapper and mTLS boundary pass real Odoo/gRPC/PostgreSQL tests, including two-session retry. | **PARTIAL:** describe the actual subject-string/context-map wire model; do not imply a typed or multi-hop subject schema. |
| RQ2: runtime and memory | Measure evaluator cost and allocation behavior without conflating it with transport or ERP latency. | `e243db5` evaluator: 1329–1494 ns/op, 0 B/op, 0 allocs/op; dense 10k candidates: 1286–1493 µs global and 1344–1485 µs same-leaf, 0 allocs/op. | **PARTIAL:** the measured evaluator case is about 1.3–1.5 µs, and dense candidate iteration remains linear; no end-to-end throughput or production latency conclusion follows. |
| RQ3: guardrails | Policy-defined typed obligations route sensitive tool calls to deterministic approval or denial paths. | DSL/engine/API tests plus real Odoo non-rollback approval, mTLS, revocation, replay and concurrency evidence. | **VERIFIED for the bounded workflow:** state three operational outcomes—ALLOW, hard DENY, and DENY with `REQUIRE_HUMAN_APPROVAL`; the wire decision enum itself is ALLOW/DENY. |
| RQ4: empirical ERP evaluation | The proposed boundary is assessed for functional, security and performance behavior at one Odoo P2P checkpoint. | Seven real Odoo/PostgreSQL/mTLS-gRPC transaction cases, two-session retry and one warm purchase-confirmation comparison frozen at `e243db5`. | **VERIFIED for the stated evidence:** no full-P2P, committed-transaction, concurrent-load, general ERP speedup or production claim. |

## 2. Five-chapter writing plan

| Chapter | Content now safe to write | Content that must remain future work or a measured result |
|---|---|---|
| 1. Problem and scope | ERP tool-call risk, constrained delegation problem, the 1-hop Odoo P2P scope and RQ1–RQ4. | Claims of deployment or economic impact without empirical evidence. |
| 2. Theory and model | ABAC input tuple, constrained delegation model, SoD and deterministic-guardrail rationale. | Any assertion that a formal proof establishes a property not verified by the implementation. |
| 3. PDP architecture | Go evaluator, COW state, JWT/proof boundary, PostgreSQL policy revision, generated standard Protobuf and typed obligations. | Dynamic HR/daily-limit attenuation, universal cluster behavior, edge restore, WORM/Vector/ClickHouse audit, and universal zero-linear-scan claims. |
| 4. Odoo implementation | Repository addon design and verified transaction/concurrency results: generated client, JWT, full-tuple proof, nonce ledger, typed-obligation mapping and mTLS transport. | Do not generalize the single-PDP Odoo boundary to production deployment or full P2P coverage. |
| 5. Evaluation and conclusion | Methodology, frozen environment disclosure and reproducibly captured results. | Historical 27 ns/36.8M RPS/286.3 ns/Odoo speedup claims, synthetic baselines, comparisons without equal workloads, and a production/WORM conclusion. |

## 3. Frozen evaluation set

| Dimension | Required cases | Acceptance artifact |
|---|---|---|
| Functional | Odoo allow, hard deny/SoD, approval obligation and PEP state transition. | Frozen Odoo mTLS integration output and persistent-record assertions. |
| Security | JWT/tenant checks, tampered/expired proof, replay behavior, revoked grant and PDP outage. | Go negative tests plus frozen Odoo cases; durable revocation has separate local three-replica PostgreSQL evidence. |
| Performance | Evaluator, dense candidates, local application path and Odoo ORM-to-PDP boundary are reported separately. | Versioned benchmark results and raw Odoo samples; no single production latency/throughput figure. |
| Comparative | Equal low-value purchase-confirmation operation and data volume for PDP PEP versus native Odoo. | Frozen methodology and 1,500 raw observations; no third-party comparison claim. |

## 4. Source map

| Thesis element | Authoritative source |
|---|---|
| Proposal objectives and RQs | [`DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md`](../thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md) |
| Scope/evidence reconciliation | [`THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`](../thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) |
| Current implementation truth | [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md) |
| Remaining work and exit criteria | [`IMPLEMENTATION_TASK_BOARD.md`](./IMPLEMENTATION_TASK_BOARD.md) |
| Canonical wire contract | [`proto/v1/policy.proto`](../../proto/v1/policy.proto) and [`PROTOCOL_CONTRACT.md`](./PROTOCOL_CONTRACT.md) |
| Odoo PEP implementation | [`custom_addons/pdp_authorizer`](../../custom_addons/pdp_authorizer) |
| Historical Odoo baseline | `E:\Projects\ERP_Mastery_Hub\02_Project_2_Odoo_Go_PDP_Approval\custom_addons\pdp_authorizer` |

## 5. Chapter 3 draft — Proposed PDP and constrained delegation design

### 3.1 Scope and design objective

This work implements a delegation-aware authorization boundary for an AI agent's purchase-order confirmation tool call in Odoo 17. The boundary is limited to Procure-to-Pay, one human-to-agent delegation hop and a repository-owned Odoo PEP. It does not train an AI model, replace the ERP, integrate SAP, or claim a general multi-agent authorization framework.

The PDP input remains the ABAC tuple `(subject, action, resource, context)`. For a delegated call, `subject` is the agent identity and the context carries the grant identifier, delegator, validity interval, nonce and protected business attributes. This is an interoperable subject-string plus context-map contract, not a typed unified multi-hop subject schema. RQ1 is therefore answered for the bounded 1-hop Odoo boundary, not for the broader multi-level formulation in the original proposal.

### 3.2 Protected delegation contract

The PEP constructs a versioned, length-prefixed HMAC-SHA256 proof over the tenant, grant, delegator, agent, action, resource, amount, delegation chain, resource creator, tool context, execution mode, nonce, issue time and expiry. The Go PDP binds the JWT identity and tenant first, checks revocation state, then verifies this tuple before evaluating policy. Thus a caller cannot substitute a protected action, resource, amount, chain, creator, tool context or nonce after the proof is issued.

The contract restricts the chain to exactly `delegator,agent`. HMAC establishes integrity and authenticity for the configured key holder; it does not provide non-repudiation. The claim is supported by proof unit tests, interceptor negative tests, the cross-language vector and the Odoo mTLS tampered-proof case; the detailed trace is maintained in [`EVALUATION_MATRIX.md`](./EVALUATION_MATRIX.md).

### 3.3 Policy decision and guardrails

The PDP evaluates policy after the security boundary accepts the delegated request. It uses deny-by-default and forbid-overrides semantics. The P2P policy denies approval when `delegation_chain contains resource.creator_id`, preventing both direct self-approval and the tested confused-deputy case in which the delegator created the purchase order.

The thesis reports three *business outcomes*: (1) `ALLOW`, which permits normal confirmation; (2) hard `DENY`, which fails the transaction; and (3) `DENY` carrying the typed `REQUIRE_HUMAN_APPROVAL` obligation, which routes the order to human review. The protocol decision enum is ALLOW/DENY; human approval is an obligation, not a third protobuf decision enum. This distinction keeps PDP and Odoo workflow semantics consistent.

### 3.4 Explicit design boundary

The implementation verifies active grant state, proof TTL, policy guardrails, SoD and revocation at its defined boundary. It does **not** consume a trusted current HR status or daily approval-limit signal. Immediate authority collapse after suspension, departure or limit exhaustion is therefore design intent only and marked **NOT VERIFIED**. External append-only retention, WORM and CloudTrail/AWS evidence are outside this experiment and **DEFERRED UNTIL 2029**.

## 6. Chapter 4 draft — Odoo 17 PEP implementation

### 4.1 PEP workflow

The repository-owned `pdp_authorizer` addon extends `purchase.order`. For a delegated confirmation, it validates the active grant, creates the protected tuple, obtains a JWT for the actor, and sends `CheckAccess` through the generated Python gRPC client over mTLS. The testbed verifies that a missing client certificate and a wrong hostname are rejected before the valid Odoo client reaches the JWT boundary.

Before the RPC, the addon serializes nonce creation with `SELECT ... FOR UPDATE` on the purchase-order row. It persists a `pdp.authorization.attempt` record unique on `(tenant_id, delegation_grant_id, delegation_nonce)` with a fingerprint of the protected business command. Reuse for a different order or command fails closed; reuse of the same completed command is idempotent. This is the concrete replay-protection contribution at the Odoo PEP/PDP boundary.

### 4.2 Decision-to-transaction mapping

| PDP result at the PEP | Odoo action | Tested transaction outcome |
|---|---|---|
| `ALLOW` | Set `pdp_status=allow`, call the base `button_confirm`, then mark the attempt executed. | One purchase confirmation; a repeated same command leaves one authorization attempt. |
| `DENY` + `REQUIRE_HUMAN_APPROVAL` | Set `state=to approve`, set `pdp_status=require_approval`, create one activity and mark the attempt approval-required. | The approval state and activity persist without rollback. |
| Hard `DENY` or unreachable PDP | Raise `AccessError`. | The enclosing Odoo transaction rolls back; the failed case does not consume the nonce/attempt. |

The non-rollback approval branch is intentional: it preserves human-review state rather than treating a controlled escalation as an error. In contrast, SoD, tampering, replay and PDP-unavailable paths fail closed and roll back the rejected transaction.

### 4.3 Functional and security evidence

The frozen mTLS testbed run on `e243db5` traversed Odoo 17 ORM, generated Python gRPC client, Go PDP and PostgreSQL 15. It reported zero failures/errors across seven transaction cases and a separately bounded two-session retry. The cases cover ALLOW idempotency, non-rollback human approval, SoD rollback, PDP outage, live revocation, tampered-proof denial and altered-command nonce replay. Two independent Odoo/PostgreSQL sessions produced one nonce, one executed attempt and a final `purchase` state after one serialization retry.

This proves stated repository testbed behavior, including a live revoke against one PDP. It does not turn the Odoo run into evidence of a remote deployment or universal multi-replica behavior. PostgreSQL three-replica propagation and restart are separate local integration evidence with 108 observations and a recorded maximum of 38.8256 ms under the 5 s test SLO. See [`ODOO_E2E_2026_09_19.md`](./evidence/ODOO_E2E_2026_09_19.md) and [`SECURITY_INVARIANTS.md`](./SECURITY_INVARIANTS.md).

## 7. Chapter 5 draft — Evaluation, RQ answers and limitations

### 5.1 Method

The evaluation uses separate boundaries rather than a single blended speed claim:

1. In-process Go tests check policy semantics, tenant/JWT binding, proof tampering, expiry, SoD and revocation.
2. The Odoo mTLS testbed checks the real ORM-to-PDP-to-PostgreSQL transaction boundary and nonce/retry behavior.
3. The performance comparison measures 750 warm low-value purchase confirmations per path (three samples of 250), comparing native Odoo/PostgreSQL with the same confirmation through the PDP PEP. Setup, warm-up and final test-harness transaction commit are excluded.

This method supports a bounded implementation evaluation. It does not measure a full P2P workflow, committed database latency, concurrent Odoo load, a production network, or an OPA/Cedar comparison; no fair equal workload exists for the latter.

### 5.2 Functional and security results

| Evaluated property | Result at `e243db5` | Interpretation |
|---|---|---|
| Full-tuple proof and TTL | Tampering, missing protected fields and expiry fail before policy evaluation. | VERIFIED for the enumerated tuple and negative tests. |
| Replay and two-session contention | Altered-command replay is denied; two sessions yield one nonce and one executed attempt. | VERIFIED for one purchase-order command, not general distributed idempotency. |
| SoD and fail-closed outage | Self/delegator-created-order approval is denied; unavailable PDP rolls back without consuming a nonce. | VERIFIED for the tested P2P policy and PEP path. |
| Revocation | Live Odoo revoke denies at one PDP; local three-replica PostgreSQL test records 108 samples, max 38.8256 ms. | VERIFIED only for stated local tests; not instant or production cluster-wide. |
| Human approval | An over-limit order enters `to approve` with one activity and one approval-required attempt. | VERIFIED for the bounded Odoo workflow. |

These results answer RQ3 positively for deterministic routing at the tested boundary, while retaining actual representation: hard deny versus deny with an approval obligation. They also provide the functional/security component of RQ4.

### 5.3 Performance results

| Boundary and workload | Frozen result | Valid conclusion |
|---|---:|---|
| Narrow in-memory evaluator | 1329–1494 ns/op; 0 B/op; 0 allocs/op | Allocation-free for measured cases, but approximately 1.3–1.5 µs, not a general sub-microsecond claim. |
| Dense 10k candidates | 1286–1493 µs global; 1344–1485 µs same-leaf; 0 allocs/op | Candidate iteration is a measured linear-scan limitation. |
| Native Odoo purchase confirmation | mean 29.933 ms; p50 28.641 ms; p95 40.807 ms; p99 48.843 ms | Reference for one warm, non-committed checkpoint only. |
| Odoo PEP + mTLS gRPC confirmation | mean 71.525 ms; p50 66.719 ms; p95 87.975 ms; p99 101.640 ms | PDP path is slower in this workload; no ERP speedup or tail-latency win is claimed. |

The performance data makes RQ2 and the performance part of RQ4 **PARTIAL**: zero allocation is reproduced for named evaluator and dense benchmarks, but the runtime target is not established across all workloads and the ERP boundary has a measured overhead. The raw method, distributions and exclusions are recorded in [`ODOO_ORM_COMPARISON_2026_09_15.md`](./evidence/ODOO_ORM_COMPARISON_2026_09_15.md).

### 5.4 Conclusion and threats to validity

The demonstrated contribution is a reproducible authorization boundary for a bounded AI-agent P2P tool call: tenant-bound identity, proof-bound 1-hop delegation, replay-aware Odoo execution, SoD, revocation checks and non-rollback human escalation are combined and tested at the Odoo PEP/PDP boundary. It is a prototype result, not evidence that the entire ERP, a general AI-agent platform or a production PDP is ready.

Important threats to validity are the single warm checkpoint, excluded final commit, testbed environment, lack of concurrent Odoo load, linear dense-policy behavior, and no trusted dynamic HR/limit attribute. A 2029 defense freeze must pin its final environment and re-run the evidence suite before any final deployment, retention or cloud-operational claim is made.
