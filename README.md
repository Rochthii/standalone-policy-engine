# Standalone In-Memory Policy Decision Point (PDP)
### Delegation-Aware Authorization & Guardrails for ERP AI Agents (Odoo 17)

> **Current implementation status (audit update 2026-09-12):** The in-memory core is verified for the measured benchmark cases, and the repository-owned Odoo PEP passes seven real ORM/mTLS-gRPC/PostgreSQL transaction cases plus a two-session concurrency/retry case. The full distributed PDP/Odoo system is still **not production-ready**: durable multi-replica revocation, audit cryptography, remote CI and deployment gates remain open. See [`CURRENT_STATE_AUDIT.md`](./docs/technical-spec/CURRENT_STATE_AUDIT.md) and [`PRODUCTION_READINESS_CHECKLIST.md`](./docs/technical-spec/PRODUCTION_READINESS_CHECKLIST.md).

**Author:** Chăm Rốch Thi  
**Affiliation:** Posts and Telecommunications Institute of Technology (PTIT)  
**Thesis:** Software Engineering Master / Graduation Thesis  

[![CI](https://github.com/Rochthii/standalone-policy-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/Rochthii/standalone-policy-engine/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/tag/Rochthii/standalone-policy-engine?label=release&color=green)](https://github.com/Rochthii/standalone-policy-engine/releases/tag/v1.0.0-core-verified)
[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Verification](https://img.shields.io/badge/7%2F7%20Vectors-PASS-brightgreen.svg)](./tests/e2e_delegation_test.go)

An in-memory Policy Decision Point (PDP) research prototype in Go implementing PBAC/ABAC, constrained delegation and deterministic ERP AI-agent guardrails. The measured core path is zero-allocation for the audited cases. The Odoo non-rollback and database-level nonce/retry workflows are verified for the listed real-boundary cases; distributed TOCTOU safety and the remaining release gates are still open.

---

## Key Highlights & Audited Core Benchmark Results

**Audited on 2026-09-11:** 13th Gen Intel Core i7-13700H (20 logical CPUs), Go 1.26.4, Windows/amd64. These values measure the in-memory `Engine` only; they do not include JWT, HMAC, GC tracking, Prometheus, audit logging, gRPC serialization, TLS, network, or Odoo.

| Scenario / Benchmark | Latency | Allocation | Throughput / Speedup |
|---|---|---|---|
| **Hot-Path Decision Latency** (`BenchmarkEvaluatorLatency`) | **390.3–492.8 ns/op** | **0 B/op, 0 allocs/op** | Three 1-second samples; narrow in-memory path |
| **Concurrent Hot-Path Load** (`BenchmarkConcurrentLoad`) | **26.74–27.51 ns/op** | **0 B/op, 0 allocs/op** | Aggregate throughput-normalized result, not per-request latency |
| **10,000 Policies Concurrent Contention** | **34.30–67.11 ns/op** | **0 B/op, 0 allocs/op** | Three 1-second samples; narrow in-memory path |
| **Deep DAG + Heavy ABAC** | **839.8–845.1 ns/op** | **0 B/op, 0 allocs/op** | Three 1-second samples; narrow in-memory path |

### Synthetic Timing Model: Odoo Native ORM vs Standalone Go PDP

`tests/baseline_odoo_orm_benchmark.py` currently uses a configured `time.sleep` for the Odoo side and a hardcoded PDP value. The table below is retained as a historical illustrative model; it is **not empirical Odoo evidence** and must not be cited as a measured speedup. See the [current-state audit](./docs/technical-spec/CURRENT_STATE_AUDIT.md).

| Evaluation Criteria | Odoo 17 Native ORM (`ir.rule`) | Standalone Go PDP (In-Memory) | Superiority Factor |
|---|---|---|:---:|
| **Illustrative Mean Latency** | **Simulated 23.77 ms** | **Hardcoded 0.000540 ms** | **Not experimentally validated** |
| **RAM Allocation on Hot-Path** | ~24 KB / query (ORM Objects) | **0 B / op (Zero-Alloc)** | **Zero GC Pressure** |
| **Heap Allocations per Check** | ~120 allocs / check | **0 allocs / op** | **Zero Memory Leaks** |
| **Anti-TOCTOU Defense** | Vulnerable (Waits for DB commit) | **O(1) tenant-scoped local revocation** | Cluster propagation/restart durability pending |
| **AI Delegation Chain Support** | Not supported (User ID only) | **Versioned full-tuple direct delegation HMAC with key-ring rotation** | Odoo duplicate, altered-command replay, rollback and two-session retry cases pass |
| **Runtime Obligation Handling** | Triggers Database Rollback Trap | **Typed PDP obligation output** | Real Odoo test verifies `to approve` plus one Activity without rollback |

---

## Architecture & Core Data Flow

```mermaid
flowchart TD
    Client(["Autonomous AI Agent / Odoo User"])
    PEP["Odoo 17 PEP Addon\n(custom_addons/pdp_authorizer)\nNon-Rollback State Machine"]
    PDP["Go PDP Server (:50051)\ngRPC CheckAccess / RevokeDelegation"]

    subgraph Layer1 ["Layer 1: Security Interceptor (< 2 µs)"]
        RevMap["In-Memory RevocationMap O(1)\nsync.Map (Anti-TOCTOU)"]
        HMAC["HMAC-SHA256 Canonical String\nProof & TTL Verification"]
        TenantIso["Tenant Isolation\nclaims.tenant_id == req.tenant_id"]
        FastDeny["Fast DENY / 403\nShort-Circuit Exit"]
    end

    subgraph Layer2 ["Layer 2: In-Memory Engine (Lock-Free COW)"]
        Trie["Multi-Level Trie O(log N)\nFNV-1a 64-bit uint64 Index"]
        DAG["Role Hierarchy DAG\nPre-computed Transitive Closure O(1)"]
        AST["Zero-Alloc AST Evaluator\nToán tử SoD contains + Bitmask IP"]
    end

    Postgres[("PostgreSQL 15+\nTransactional Sequence\ntenants.revision")]

    Client -->|"Calls Tool / button_confirm()"| PEP
    PEP -->|"Standard ORM Transaction"| Postgres
    PEP -->|"gRPC with HMAC Proof"| PDP
    PDP --> Layer1
    Layer1 -->|"Pass: Proof Valid"| Layer2
    Layer1 -->|"Fail: Tampered / Expired / Revoked"| FastDeny
    FastDeny -->|"Early Response (403 / DENY)"| PDP
    Layer2 -->|"ALLOW / DENY + Obligations"| PDP
    PDP -->|"Decision Response"| PEP
    PEP -->|"state -> 'to approve' (No Rollback)"| Client

    PEP -->|"action_revoke() Sync gRPC"| PDP
    PDP -->|"Instant Write O(1)"| RevMap

    PDP -.->|"Async Ring Buffer (pgx.CopyFrom)"| Postgres
    Postgres -->|"LISTEN/NOTIFY Gap Catch-Up (< 50ms)"| PDP

    style PDP fill:#1e3a5f,color:#fff
    style Layer1 fill:#4a154b,color:#fff
    style Layer2 fill:#0f2d25,color:#fff
    style FastDeny fill:#7a1c1c,color:#fff
```


---

## Verified 7/7 In-Process Delegation Vectors

The Go test fixture passes all seven logic vectors defined in [`tests/e2e_delegation_test.go`](./tests/e2e_delegation_test.go). Those seven tests remain in-process evidence. A separate Docker gate verifies seven real Odoo transaction cases plus a two-session concurrency/retry case across ORM, network transport and PostgreSQL; neither suite proves multi-replica revocation.

| Vector ID | Test Scenario Description | Expected Decision | Result |
|---|---|:---:|:---:|
| **`TC-01`** | Manager creates PO and attempts to self-approve | **`DENY`** | **PASS** |
| **`TC-02`** | AI Agent attempts to approve PO created by delegating Manager (SoD Chain) | **`DENY`** | **PASS** |
| **`TC-03`** | AI Agent autonomously approves PO within delegated limit (<= $2,000) | **`ALLOW`** | **PASS** |
| **`TC-04`** | AI Agent attempts PO approval above limit (> $2,000, Guardrail Ceiling) | **`DENY`** + `REQUIRE_HUMAN_APPROVAL` | **PASS** |
| **`TC-05`** | Malicious Actor tampers with PO amount in context | **`403 PermissionDenied`** | **PASS** |
| **`TC-06`** | Manager revokes delegation on Odoo; Agent calls immediately (Anti-TOCTOU) | **`DENY`** (`POL-REVOCATION-BLACK-LIST`) | **PASS** |
| **`TC-07`** | AI Agent presents delegation proof with expired TTL token | **`403 PermissionDenied`** | **PASS** |

---

## Quick Start

### 1. Docker Testbed Status

The Compose file uses only repository-local Odoo addon and generated-client inputs for this path. On 2026-09-12 its fresh-database gate passed 7/7 transaction cases and the two-session concurrency/retry assertion over mTLS; a missing-client-certificate probe was rejected before RPC handling. It remains a development testbed—not a frozen release environment—because base images use mutable tags and remote CI evidence is still open. See the [evidence record](./docs/technical-spec/evidence/ODOO_E2E_2026_09_12.md).

```bash
# Clone the repository
git clone https://github.com/Rochthii/standalone-policy-engine.git
cd standalone-policy-engine

# Run the real Odoo/PostgreSQL/gRPC transaction gate
make test-odoo-e2e
```

### 2. Run Locally from Source
```bash
# 1. Run Core Engine & Layer 1 Interceptor Tests
go test -v ./internal/security ./internal/server

# 2. Verify all 7 E2E Delegation Vectors
go test -v ./tests -run=TestE2E_P2P_Delegation_7Vectors

# 3. Run Sub-Microsecond Evaluator Benchmark
go test -bench=BenchmarkEvaluatorLatency -benchmem ./tests -run=^$

# 4. Verify the Odoo/PDP proof compatibility vector
python custom_addons/pdp_authorizer/tests/test_pdp_protocol.py
```

---

## Cedar-like Declarative DSL (P2P Seed Rules)

Rules in [`configs/policies.cedar`](./configs/policies.cedar) demonstrate Separation of Duties (SoD) and AI Guardrails:

```cedar
// 1. Autonomous AI Agent PO approval within delegated limit ($2,000)
permit(
    principal in role:ai_agent,
    action    == action:APPROVE,
    resource  == doc:purchase_order
)
when {
    context.amount <= 2000
};

// 2. High-value transactions trigger Human Approval obligation
forbid(
    principal in role:ai_agent,
    action    == action:APPROVE,
    resource  == doc:purchase_order
)
when {
    context.amount > 2000
};

// 3. Separation of Duties: Prevent creator and delegator from approving
forbid(
    principal == any,
    action    == action:APPROVE,
    resource  == doc:purchase_order
)
when {
    context.delegation_chain contains resource.creator_id
};
```

---

## Repository Structure

```text
standalone-policy-engine/
├── .agents/                 # AI Master Context & 9 Concise Domain Skills (< 40 lines each)
├── configs/
│   └── policies.cedar       # 6 Standard P2P seed rules enforcing SoD via contains
├── cmd/
│   ├── pdp-server/          # gRPC Data Plane Server (:50051)
│   ├── control-plane/       # REST Control Plane API (:8080)
│   └── pectl/               # Enterprise Policy CLI
├── custom_addons/
│   └── pdp_authorizer/      # Odoo 17 PEP, proof signer and transactional nonce ledger
├── internal/
│   ├── engine/              # Multi-level Trie, Role DAG, Zero-Alloc AST Evaluator, COW
│   ├── security/            # DelegationManager (HMAC Canonical, O(1) RevocationMap, JWT)
│   ├── parser/              # Cedar DSL Lexer, Pratt Parser (Depth <= 15), Compiler
│   ├── server/              # gRPC Server (Layer 1 Interceptors), HTTP Handlers, Replay Buffer
│   ├── audit/               # Bounded async logger, redaction, pgx.CopyFrom; encryption/spill pending
│   └── storage/             # PostgreSQL pgx driver, Postgres LISTEN/NOTIFY sync, BadgerDB
├── proto/v1/                # Protobuf Contract (CheckAccess, ExplainDecision, RevokeDelegation)
├── docs/                    # Master Index & 12 Technical Specifications
│   ├── 00_MASTER_INDEX.md   # System navigation & live metrics
│   ├── technical-spec/      # ARCH_SPEC, PROTOCOL_CONTRACT, SECURITY_INVARIANTS, etc.
│   └── thesis-proposal/     # PTIT Master Thesis Proposal (5 Chapters)
├── tests/                   # 7 E2E Vectors, Baseline Benchmark, ERP ABAC test suite
├── benchmarks/              # Static 2026 test artifacts & latency reports
├── docker-compose.testbed.yml # Frozen single-command testbed (2026-2029)
├── AGENTS.md / CLAUDE.md    # Master AI context guide (Single Source of Truth)
└── CHANGELOG.md             # Semantic release history (Current: v1.15.0)
```

---

## CLI Tool (pectl)

`pectl` provides a developer CLI for policy management, dry-run simulation, and live checking:

```bash
# Check live access permission
pectl check tenant-odoo --subject agent:procurement_copilot --action APPROVE --resource doc:purchase_order

# Dry-run simulate a draft policy
pectl simulate tenant-odoo --subject user:manager_bob --action APPROVE --resource doc:purchase_order --draft-file draft.cedar
```

---

## License & Academic Attribution

Distributed under the **MIT License**.

**Author:** Chăm Rốch Thi  
**Institution:** Posts and Telecommunications Institute of Technology (PTIT)  

This project serves as the primary implementation and experimental testbed for the **PTIT Software Engineering Master / Graduation Thesis**:
> *"XÂY DỰNG CƠ CHẾ POLICY DECISION POINT HỖ TRỢ ỦY QUYỀN CÓ KIỂM SOÁT (DELEGATION-AWARE AUTHORIZATION) CHO TÁC TỬ AI TRONG HỆ THỐNG ERP — NGHIÊN CỨU TRIỂN KHAI VÀ ĐÁNH GIÁ THỰC NGHIỆM TRÊN NỀN TẢNG ODOO"*.
