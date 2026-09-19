# EVALUATION_MATRIX.md — Verification Matrix & Performance Benchmarks

> **Audit notice (2026-09-11):** Bảng này kết hợp mục tiêu thiết kế, kết quả test in-process và một số số liệu chưa có bằng chứng E2E tương ứng. Trạng thái xác minh chính thức nằm trong [`CURRENT_STATE_AUDIT.md`](./CURRENT_STATE_AUDIT.md). Không coi obligation, Odoo PEP, cluster-wide revocation hoặc gRPC mTLS latency là verified cho tới khi checklist production tương ứng PASS.

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

## 2. Comparative Benchmark Matrix (current evidence)

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

## 3. Performance Target Metrics & Budgets

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ PERFORMANCE BUDGET & MEASURED BENCHMARKS (20 CORES CPU, 32GB RAM)           │
├──────────────────────────────────────┬──────────────────┬───────────────────┤
│ Metric Parameter                     │ Target Budget    │ Measured Result   │
├──────────────────────────────────────┼──────────────────┼───────────────────┤
│ Evaluator narrow cases               │ target-specific  │ 603–1144 ns/op, 0 allocs/op |
│ Dense 10k candidate cases             │ diagnostic       │ global 662–1051 µs; same-leaf 1144–1234 µs; 0 allocs/op |
│ Local full path                       │ boundary-specific│ p50 337–616 µs; p99 1.443–2.123 ms |
│ Odoo purchase confirmation            │ boundary-specific│ native 29.933 ms; PDP 71.525 ms mean |
│ Production-ready / WORM               │ no current claim │ DEFERRED / NO-GO |
└──────────────────────────────────────┴──────────────────┴───────────────────┘
```
