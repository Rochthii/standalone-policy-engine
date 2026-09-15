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
| **S6** | **Prompt Injection Extreme Value Attack** | `subject: "agent:po_bot"`<br>`amount: "10000000"` (\$10M)<br>`execution_mode: "autonomous"` | `POL-AGENT-AUTONOMOUS-HIGH-FORBID` | `DENY` | `["REQUIRE_HUMAN_APPROVAL"]` | Evaluated in **286.3 ns**<br>Immediate isolation<br>Hard exception |
| **S7** | **Generalized SoD Bypass Attack** | `subject: "agent:po_bot"`<br>`delegation: "user:bob,agent:po_bot"`<br>`creator: "user:bob"` (Delegator created order) | `POL-SOD-GENERALIZED-FORBID` | `DENY` | None | Detected via `contains`<br>Raises `AccessError`<br>**FULL ROLLBACK** |

---

## 2. Comparative Benchmark Matrix (current evidence)

> The former fixed 10–50 ms Odoo, OPA, and 30,000x comparison has no like-for-like evidence and is retired. The only Odoo comparison below is a measured, narrow authorization workload; no OPA measurement is currently claimed.

| Measured workload (commit `4bb4c48`) | Odoo native `ir.rule` + PostgreSQL | Odoo client + PDP mTLS gRPC | Interpretation |
|---|---:|---:|---|
| Mean latency, 750 warm authorization checks | 1.395756 ms | 1.036841 ms | PDP path is 1.346x lower in this workload only |
| p50 latency | 1.364612 ms | 0.941398 ms | PDP path is 1.450x lower |
| p95 latency | 1.873552 ms | 1.807840 ms | Near parity (1.036x) |
| p99 latency | 2.412293 ms | 2.884821 ms | Native path is lower; no tail-latency win is claimed |
| Scope | One existing low-value purchase order; setup/warm-up/mutations excluded | Same authorization intent; JWT, full proof, mTLS gRPC included | Not a full ERP transaction, concurrent load, allocation, or production comparison |

Raw samples and method limits: [`evidence/ODOO_ORM_COMPARISON_2026_09_15.md`](./evidence/ODOO_ORM_COMPARISON_2026_09_15.md).

---

## 3. Performance Target Metrics & Budgets

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ PERFORMANCE BUDGET & MEASURED BENCHMARKS (20 CORES CPU, 32GB RAM)           │
├──────────────────────────────────────┬──────────────────┬───────────────────┤
│ Metric Parameter                     │ Target Budget    │ Measured Result   │
├──────────────────────────────────────┼──────────────────┼───────────────────┤
│ Hot-Path In-Memory Latency (1 Policy)│ < 100 ns         │ 27.12 ns / op     │
│ High-Density Workload (10k Policies) │ < 200 ns         │ 35.94 ns / op     │
│ Complex AI Guardrail + Obligations   │ < 500 ns         │ 286.3 ns / op     │
│ Deep Role DAG (11 levels) + 5k Decoys│ < 1,500 ns       │ 810.9 ns / op     │
├──────────────────────────────────────┼──────────────────┼───────────────────┤
│ End-to-End Latency (gRPC mTLS)       │ < 1.0 ms         │ 0.31 ms – 0.35 ms │
├──────────────────────────────────────┼──────────────────┼───────────────────┤
│ Memory Allocations on Hot-Path       │ 0 B / op         │ 0 B / op          │
│ Allocation Count on Hot-Path         │ 0 allocs / op    │ 0 allocs / op     │
├──────────────────────────────────────┼──────────────────┼───────────────────┤
│ In-Memory Peak Throughput (20 Cores) │ > 10,000,000 RPS │ 36,800,000 RPS    │
│ Delegation Revocation Propagation    │ < 10 µs          │ < 1.0 µs          │
└──────────────────────────────────────┴──────────────────┴───────────────────┘
```
