# Thesis Chapter Mapping and Evidence Plan

> **Status:** 2026-09-12. This is an evidence plan for the five-chapter thesis, not a declaration that every planned experiment or Odoo integration has been completed. Read [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md) and [`THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md`](../thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md) first.

## 1. Research-question evidence matrix

| RQ | Thesis claim to establish | Available evidence | Evidence still required |
|---|---|---|---|
| RQ1: unified subject and delegation | A policy request can distinguish authenticated human/service identity, AI agent context and a constrained 1-hop delegation. | Tenant-bound JWT validation, full-tuple proof, migrated mapper and mTLS boundary are verified across real Odoo/gRPC/PostgreSQL transactions, including two-session retry. | Freeze the evidence at one current commit and describe the actual subject-string/context-map wire model; do not imply typed multi-hop protobuf support. |
| RQ2: runtime and memory | The evaluator remains sub-microsecond with zero allocations for stated in-memory cases. | 2026-09-12 local three-sample evaluator result: 390.3–492.8 ns/op, 0 B/op, 0 allocs/op; 2026-09-14 dense 10,000-candidate samples: 372–389 µs/op, raw-key collision guards; local TCP application-path p50 200.5–207.7 µs and p99 723.4–890.8 µs across JWT/proof/gRPC/metrics/audit queueing. | Release-commit environment capture and measurements for production boundaries excluded from the local path. |
| RQ3: guardrails | Policy-defined typed obligations can route sensitive AI tool calls to deterministic approval or deny paths. | DSL/engine/API tests plus real Odoo non-rollback approval, mTLS, revocation, replay and concurrency evidence. | State the result as three operational outcomes: ALLOW, hard DENY, and DENY with approval obligation. |
| RQ4: empirical ERP evaluation | The proposed system is correct, secure and performant for a bounded Odoo P2P authorization checkpoint. | Go tests, seven real Odoo/PostgreSQL/gRPC transaction cases, two-session retry and one warm purchase-confirmation comparison. | Current-HEAD evidence freeze; no full-P2P, committed-transaction, concurrent-load or general ERP speedup claim. |

## 2. Five-chapter writing plan

| Chapter | Content now safe to write | Content that must remain future work or a measured result |
|---|---|---|
| 1. Problem and scope | ERP tool-call risk, constrained delegation problem, the 1-hop Odoo P2P scope and RQ1–RQ4. | Claims of deployment or economic impact without empirical evidence. |
| 2. Theory and model | ABAC input tuple, constrained delegation model, SoD and deterministic-guardrail rationale. | Any assertion that a formal proof establishes a property not verified by the implementation. |
| 3. PDP architecture | Go evaluator, COW state, JWT/proof boundary, PostgreSQL policy revision, generated standard Protobuf and typed obligations. | Cluster-wide revocation, edge restore, WORM/Vector/ClickHouse audit, and universal zero-linear-scan claims. |
| 4. Odoo implementation | Repository addon design and verified transaction/concurrency results: generated client, JWT, full-tuple proof, nonce ledger, typed-obligation mapping and mTLS transport. | Do not generalize the single-PDP Odoo boundary to production deployment or full P2P coverage. |
| 5. Evaluation and conclusion | Methodology, environment disclosure and results that are reproducibly captured. | Historical 27 ns/36.8M RPS/286.3 ns/Odoo speedup claims, synthetic baselines and comparisons without equal workloads. |

## 3. Required final evaluation set

| Dimension | Required cases | Acceptance artifact |
|---|---|---|
| Functional | Odoo allow, hard deny/SoD, approval obligation and PEP state transition. | Odoo integration test output and persistent-record assertions. |
| Security | Missing/invalid JWT, cross-tenant request, tampered proof, expired proof, replay behavior, revoked grant and PDP outage. | Negative integration tests; replay and durable-revocation tests after those features are implemented. |
| Performance | Evaluator, authenticated gRPC, and Odoo ORM-to-PDP end-to-end under the same published hardware/load profile. | Raw benchmark samples, allocations, percentiles, throughput, CPU/RSS/GC and commit SHA. |
| Comparative | Equal business operation and data volume for the PDP path and Odoo native path; optional third-party comparison only if reproducible. | Methodology, scripts and raw results. |

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
