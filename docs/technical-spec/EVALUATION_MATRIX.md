# EVALUATION_MATRIX.md — Verification Matrix & Performance Benchmarks

> **Evidence notice (2026-09-19):** Trạng thái xác minh chính thức nằm trong [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md). Bảng dưới đây tách design intent, code/test và evidence đã freeze; không suy diễn thành production, WORM, latency mTLS end-to-end tổng quát hoặc universal cluster behavior.

## 1. Edge Test Scenarios (7 Scenarios)

| # | Scenario Name | Input Conditions ($\mathcal{I} = \langle \mathcal{S}, \mathcal{A}, \mathcal{R}, \mathcal{C} \rangle$) | Matching Policy ID | Evaluator Decision ($\mathcal{D}$) | Runtime Obligations ($\mathcal{O}$) | PEP Handling in Odoo (State vs. Rollback) |
|---|---|---|---|---|---|---|
| **S1** | **Standard Human Approval** | `subject: "user:bob"`<br>`action: APPROVE_PO`<br>`amount: "1500"` ($\le \$5,000$ limit)<br>`creator: "user:alice"` | `POL-USER-DEPT-APPROVE-PERMIT` | `ALLOW` | None | `order.pdp_status = 'allow'`<br>Calls `super().button_confirm()`<br>State: `purchase` |
| **S2** | **Human Over-Limit Escalation** | `subject: "user:bob"`<br>`action: APPROVE_PO`<br>`amount: "12000"` ($> \$5,000$ limit)<br>`creator: "user:alice"` | `POL-USER-OVERLIMIT-FORBID` | `DENY` | `["REQUIRE_HUMAN_APPROVAL"]` | `order.write({'state': 'to approve'})`<br>Generates Activity for Director<br>**NO ROLLBACK** |
| **S3** | **Classic SoD Violation (Self-Approval)** | `subject: "user:alice"`<br>`action: APPROVE_PO`<br>`creator: "user:alice"` | `POL-SOD-FORBID-SELF-APPROVAL` | `DENY` | None | `order.write({'pdp_status': 'deny'})`<br>Raises `AccessError`<br>**FULL ROLLBACK** |
| **S4** | **Autonomous AI Low-Value PO** | `subject: "agent:po_bot"`<br>`tool: "tool:auto_confirm_po"`<br>`amount: "800"` ($\le \$2,000$ limit)<br>`creator: "user:alice"` | `POL-AGENT-AUTONOMOUS-PERMIT` | `ALLOW` | None | `order.pdp_status = 'allow'`<br>Calls `super().button_confirm()`<br>State: `purchase` |
| **S5** | **Delegated AI High-Value PO (Human-in-the-Loop)** | `subject: "agent:po_bot"`<br>`amount: "50000"` ($> \$2,000$ limit)<br>`delegation: "user:bob,agent:po_bot"`<br>`creator: "user:alice"` | `POL-AGENT-AUTONOMOUS-HIGH-FORBID` | `DENY` | `["REQUIRE_HUMAN_APPROVAL"]` | `order.write({'state': 'to approve'})`<br>Generates Activity for CFO<br>**NO ROLLBACK** |
| **S6** | **Out-of-scope high-value AI tool-call** | `subject: "agent:po_bot"`<br>`amount: "10000000"` (\$10M)<br>`execution_mode: "autonomous"` | `POL-AGENT-AUTONOMOUS-HIGH-FORBID` | `DENY` | `["REQUIRE_HUMAN_APPROVAL"]` | Deterministic policy denial/approval routing; no claim about an actual LLM prompt-injection runtime or fixed latency |
| **S7** | **Generalized SoD Bypass Attack** | `subject: "agent:po_bot"`<br>`delegation: "user:bob,agent:po_bot"`<br>`creator: "user:bob"` (Delegator created order) | `POL-SOD-GENERALIZED-FORBID` | `DENY` | None | Detected via `contains`<br>Raises `AccessError`<br>**FULL ROLLBACK** |

---

## 2. Delegation security and effectiveness matrix

| Property evaluated | Implemented control | Code and reproducible test | Frozen evidence | Status and thesis boundary |
|---|---|---|---|---|
| Trusted principal and tenant binding | JWT claims replace caller-supplied subject; tenant must match the request | `internal/server/grpc_server.go`; `tests/grpc_auth_test.go` | Odoo mTLS boundary reaches JWT validation | **VERIFIED** for the tested PDP boundary; it is not an enterprise IAM integration claim. |
| 1-hop full-tuple integrity | Versioned, length-prefixed HMAC binds tenant, grant, delegator, agent, action, resource, amount, chain, creator, tool, execution mode, nonce and validity window | `internal/security/delegation_proof.go`; `TestDelegationManager_VerifyProof_BindsCompleteTuple`; `TestDelegationInterceptor_CompleteTupleIsRequired` | Odoo mTLS E2E includes a tampered-proof denial | **VERIFIED** for proof integrity/authenticity. HMAC is not non-repudiation and the protocol is deliberately 1-hop. |
| TTL and proof tampering fail closed | Missing, expired or modified protected fields return `PermissionDenied` before policy evaluation | `internal/server/delegation_auth.go`; `TestDelegationInterceptor_ExpiredProof_Denied`; `TestDelegationInterceptor_TamperedProof_Denied` | Odoo mTLS E2E: tampered proof denial | **VERIFIED** for the listed negative cases. |
| Replay protection and idempotent command outcome | Odoo locks the PO nonce and persists a unique `(tenant, grant, nonce)` authorization attempt with a protected command fingerprint | `custom_addons/pdp_authorizer/models/purchase_order.py`; `test_nonce_replay_for_different_order_fails_closed` | Odoo mTLS E2E: altered-command replay denied; two independent sessions finish with one nonce and one executed attempt | **VERIFIED** for one purchase-order command and the tested two-session contention; not a general distributed idempotency claim. |
| Separation of duties | A policy `contains` check denies when the creator is in the delegated approval chain | `internal/engine/evaluator.go`; `TestE2E_P2P_Delegation_7Vectors` TC-01/TC-02; `test_sod_hard_deny_rolls_back_attempt_and_nonce` | In-process vectors and Odoo mTLS rollback case | **VERIFIED** for the tested P2P policy and 1-hop chain; not a universal ERP SoD model. |
| Revocation / TOCTOU mitigation | Revoked grants are checked before evaluation; PostgreSQL sync loads a durable snapshot before ready | `internal/server/grpc_server.go`; `TestDelegationInterceptor_Revocation_TOCTOU_Defense`; `TestRevocationPropagationThreeReplicasAndRestart` | Odoo mTLS E2E live revoke (one PDP); local 3-replica PostgreSQL test, 108 samples, max 38.8256 ms under a 5 s SLO | **VERIFIED FOR STATED LOCAL TESTS.** Do not call this instant, universal or production cluster-wide revocation. |
| Human-approval routing without rollback | A typed `REQUIRE_HUMAN_APPROVAL` obligation transitions the PO to `to approve`, records one attempt and schedules one activity | `custom_addons/pdp_authorizer/models/purchase_order.py`; `test_approval_obligation_commits_once_without_rollback` | Odoo mTLS E2E | **VERIFIED** for the tested purchase-confirmation workflow; it does not establish a general approval-process deployment. |
| Availability failure | Odoo raises `AccessError` when the PDP cannot be reached and the surrounding transaction rolls back | `custom_addons/pdp_authorizer/models/pdp_client.py`; `test_pdp_outage_fails_closed_without_consuming_nonce` | Odoo mTLS E2E outage case | **VERIFIED** for the tested unavailable PDP client path; not an availability SLO. |
| Dynamic attenuation from current HR/limit state | Proposed status/limit inputs would narrow the delegator's active rights | No current request field, policy input or test for `delegator_status` / `delegator_limit` | None | **NOT VERIFIED.** Exclude “immediate collapse on suspension/departure/daily limit” from results. |

The experiment's concrete contribution is therefore the enforced Odoo PEP/PDP boundary for a bounded P2P tool call: it authenticates the acting subject, protects the 1-hop delegation tuple, prevents tested replay/SoD/revocation cases, and preserves a non-rollback approval state. It is not evidence for a production authorization service or a generalized multi-agent delegation framework.

## 3. Comparative Benchmark Matrix (current evidence)

> Historical fixed-latency, OPA and speedup claims are retired. The current comparison is a measured, narrow purchase-confirmation workload; no OPA measurement is claimed.

| Measured workload (commit `e243db5`) | Native Odoo purchase confirmation + PostgreSQL | Odoo PEP purchase confirmation + mTLS gRPC | Interpretation |
|---|---:|---:|---|
| Mean latency, 750 warm purchase confirmations | 29.933 ms | 71.525 ms | PDP path is slower in this workload only |
| p50 latency | 28.641 ms | 66.719 ms | Native path is lower |
| p95 latency | 40.807 ms | 87.975 ms | No tail-latency win is claimed |
| p99 latency | 48.843 ms | 101.640 ms | Native path is lower |
| Scope | One warm purchase-confirmation checkpoint; final commit excluded | Same business mutation with JWT, full proof and mTLS gRPC | Not full P2P, committed transaction, concurrent load or production comparison |

Raw samples and method limits: [`evidence/ODOO_ORM_COMPARISON_2026_09_15.md`](./evidence/ODOO_ORM_COMPARISON_2026_09_15.md).

---

## 4. Performance Target Metrics & Budgets

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ PERFORMANCE BUDGET & MEASURED BENCHMARKS (20 CORES CPU, 32GB RAM)           │
├──────────────────────────────────────┬──────────────────┬───────────────────┤
│ Metric Parameter                     │ Target Budget    │ Measured Result   │
├──────────────────────────────────────┼──────────────────┼───────────────────┤
│ Evaluator narrow cases               │ target-specific  │ 1329–1494 ns/op, 0 allocs/op |
│ Dense 10k candidate cases             │ diagnostic       │ global 1286–1493 µs; same-leaf 1344–1485 µs; 0 allocs/op |
│ Local full path                       │ boundary-specific│ p50 337–616 µs; p99 1.443–2.123 ms |
│ Odoo purchase confirmation            │ boundary-specific│ native 29.933 ms; PDP 71.525 ms mean |
│ Production-ready / WORM               │ no current claim │ DEFERRED / NO-GO |
└──────────────────────────────────────┴──────────────────┴───────────────────┘
```
