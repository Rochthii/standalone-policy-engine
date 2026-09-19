# Standalone In-Memory Policy Decision Point (PDP)
### Delegation-Aware Authorization & Guardrails for ERP AI Agents (Odoo 17)

> **Current implementation status (evidence update 2026-09-19):** This repository is a **research prototype**, not a production-ready PDP. The authoritative status, limits and release gates are in [`CURRENT_STATE_AUDIT.md`](./docs/technical-spec/CURRENT_STATE_AUDIT.md) and [`PRODUCTION_READINESS_CHECKLIST.md`](./docs/technical-spec/PRODUCTION_READINESS_CHECKLIST.md).

**Author:** Chăm Rốch Thi  
**Affiliation:** Posts and Telecommunications Institute of Technology (PTIT)  
**Thesis:** Software Engineering Graduation Thesis

[![CI](https://github.com/Rochthii/standalone-policy-engine/actions/workflows/ci.yml/badge.svg)](https://github.com/Rochthii/standalone-policy-engine/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/tag/Rochthii/standalone-policy-engine?label=release&color=green)](https://github.com/Rochthii/standalone-policy-engine/releases/tag/v1.0.0-core-verified)
[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Verification](https://img.shields.io/badge/7%2F7%20In--Process%20Vectors-PASS-brightgreen.svg)](./tests/e2e_delegation_test.go)

An in-memory Policy Decision Point (PDP) research prototype in Go implementing PBAC/ABAC, constrained delegation and deterministic ERP AI-agent guardrails. The measured core path is zero-allocation for the audited cases. The Odoo non-rollback, mTLS and database-level nonce/retry workflows are verified for the listed real-boundary cases; multi-replica revocation is verified in a local PostgreSQL test, while production release gates remain open.

---

## Read this first

- [`CURRENT_STATE_AUDIT.md`](./docs/technical-spec/CURRENT_STATE_AUDIT.md): authoritative implementation status and claim boundaries.
- [`EVALUATION_MATRIX.md`](./docs/technical-spec/EVALUATION_MATRIX.md): thesis scenarios and comparative result summary.
- [`evidence/`](./docs/technical-spec/evidence/): reproducible commands, limits and raw benchmark artifacts.

Current source evidence is `e243db5`. It covers three distinct boundaries that must not be compared as one end-to-end SLO:

| Boundary | Current result | Limit |
|---|---|---|
| In-memory evaluator | 1329–1494 ns/op, 0 allocs | Excludes JWT, gRPC, TLS, audit and Odoo |
| Local TCP gRPC path | p50 337–616 µs; p99 1.443–2.123 ms | Excludes mTLS, PostgreSQL audit flush, containers and Odoo |
| Odoo purchase confirmation | Native mean 29.933 ms; PDP mean 71.525 ms | Warm low-value PO only; final DB commit and concurrency excluded |

The PDP path is slower in the measured Odoo workload. Raw samples and method limits are in the [Odoo evidence](./docs/technical-spec/evidence/ODOO_ORM_COMPARISON_2026_09_15.md).

---

## Architecture & Core Data Flow

```mermaid
flowchart TD
    Client(["Autonomous AI Agent / Odoo User"])
    PEP["Odoo 17 PEP Addon\n(custom_addons/pdp_authorizer)\nNon-Rollback State Machine"]
    PDP["Go PDP Server (:50051)\ngRPC CheckAccess / RevokeDelegation"]

    subgraph Layer1 ["Layer 1: Security Interceptor (boundary-specific latency)"]
        RevMap["In-Memory RevocationMap O(1)\nsync.Map (Anti-TOCTOU)"]
        HMAC["Versioned length-prefixed full-tuple HMAC\nProof & TTL Verification"]
        TenantIso["Tenant Isolation\nclaims.tenant_id == req.tenant_id"]
        FastDeny["Fast DENY / 403\nShort-Circuit Exit"]
    end

    subgraph Layer2 ["Layer 2: In-Memory Engine (Lock-Free COW)"]
        Trie["Multi-Level Trie O(log N)\nFNV-1a 64-bit uint64 Index"]
        DAG["Role Hierarchy DAG\nPre-computed Transitive Closure O(1)"]
        AST["Measured-case AST Evaluator\nToán tử SoD contains + Bitmask IP"]
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
    PDP -->|"Local O(1) update + PostgreSQL propagation"| RevMap

    PDP -.->|"Bounded async audit queue (pgx.CopyFrom)"| Postgres
    Postgres -->|"LISTEN/NOTIFY revision reconciliation"| PDP

    style PDP fill:#1e3a5f,color:#fff
    style Layer1 fill:#4a154b,color:#fff
    style Layer2 fill:#0f2d25,color:#fff
    style FastDeny fill:#7a1c1c,color:#fff
```


---

## Verified 7/7 In-Process Delegation Vectors

The Go test fixture passes all seven logic vectors defined in [`tests/e2e_delegation_test.go`](./tests/e2e_delegation_test.go). Those seven tests remain in-process evidence. A separate Docker gate verifies seven real Odoo transaction cases plus a two-session concurrency/retry case across ORM, network transport and PostgreSQL. Multi-replica revocation is evidenced separately by [`evidence/REVOCATION_DURABILITY_2026_09_13.md`](./evidence/REVOCATION_DURABILITY_2026_09_13.md).

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

The Compose file uses only repository-local Odoo addon and generated-client inputs for this path. Its external bases are manifest-digest pinned. The fresh-database Odoo gate and the combined Control Plane/PDP gate run in remote CI without a skip fallback; this is still a development testbed, not a production release environment, because the external audit-retention gate remains open. See [`CI_ODOO_2026_09_18.md`](./docs/technical-spec/evidence/CI_ODOO_2026_09_18.md) and [`REL_INTEGRATION_2026_09_18.md`](./docs/technical-spec/evidence/REL_INTEGRATION_2026_09_18.md).

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

# 2. Verify all 7 in-process delegation vectors
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
│   ├── engine/              # Multi-level Trie, Role DAG, measured-case AST evaluator, COW
│   ├── security/            # DelegationManager (full-tuple HMAC, O(1) local revocation, JWT)
│   ├── parser/              # Cedar DSL Lexer, Pratt Parser (Depth <= 15), Compiler
│   ├── server/              # gRPC Server (Layer 1 Interceptors), HTTP Handlers, Replay Buffer
│   ├── audit/               # Bounded async logger, redaction, pgx.CopyFrom, encryption/spill replay
│   └── storage/             # PostgreSQL pgx driver, Postgres LISTEN/NOTIFY sync, BadgerDB
├── proto/v1/                # Protobuf Contract (CheckAccess, ExplainDecision, RevokeDelegation)
├── docs/                    # Master Index & 12 Technical Specifications
│   ├── 00_MASTER_INDEX.md   # System navigation & live metrics
│   ├── technical-spec/      # ARCH_SPEC, PROTOCOL_CONTRACT, SECURITY_INVARIANTS, etc.
│   └── thesis-proposal/     # PTIT Graduation Thesis Proposal (5 Chapters)
├── tests/                   # 7 in-process vectors, benchmarks, ERP ABAC test suite
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

This project serves as the primary implementation and experimental testbed for the **PTIT Software Engineering Graduation Thesis**:
> *"XÂY DỰNG CƠ CHẾ POLICY DECISION POINT HỖ TRỢ ỦY QUYỀN CÓ KIỂM SOÁT (DELEGATION-AWARE AUTHORIZATION) CHO TÁC TỬ AI TRONG HỆ THỐNG ERP — NGHIÊN CỨU TRIỂN KHAI VÀ ĐÁNH GIÁ THỰC NGHIỆM TRÊN NỀN TẢNG ODOO"*.
