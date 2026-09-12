# Thesis Scope and Evidence Alignment

> **Status date:** 2026-09-12  
> **Purpose:** This document is the authoritative bridge between the thesis proposal, the Standalone Policy Engine (PDP), and the Odoo baseline in Project 2. It prevents planned architecture and historical benchmark claims from being presented as verified results.

## 1. Thesis positioning

The thesis title, four research questions, 1-hop delegation scope, and Odoo 17 Procure-to-Pay validation domain remain valid. The project is positioned as a **delegation-aware authorization prototype for AI-agent tool calls in ERP**, not as a production-ready authorization product and not as research into training an AI model.

The thesis contribution is evaluated through three separable artifacts:

| Artifact | Location | Current role |
|---|---|---|
| PDP foundation | This repository | Go policy engine, data-plane security boundary, policy synchronization and audit pipeline |
| Odoo PEP implementation | `custom_addons/pdp_authorizer` | Repository-owned migrated addon; seven mTLS transaction cases plus two-session retry verified |
| Historical Odoo baseline | `E:\Projects\ERP_Mastery_Hub\02_Project_2_Odoo_Go_PDP_Approval` | Design input only; not a release artifact |
| Thesis evidence | `docs/technical-spec/CURRENT_STATE_AUDIT.md` and reproducible test outputs | Only source for statements that an implementation or result is verified |

## 2. Current scope status

| Thesis area | Status | Evidence / required work |
|---|---|---|
| RQ1: unified subject, context and 1-hop delegation | Verified for the current single-PDP Odoo boundary | PDP accepts an authenticated principal, resource context and full-tuple proof; the mapper plus duplicate/altered/concurrent nonce behavior pass real Odoo/mTLS-gRPC/PostgreSQL tests. |
| RQ2: in-memory runtime | Verified only for measured evaluator cases | The 2026-09-12 three-sample run reports 390.3–492.8 ns/op with 0 B/op and 0 allocs/op for the measured evaluator case. This is not gRPC, mTLS, JWT or Odoo end-to-end latency. |
| RQ3: deterministic guardrails and obligations | Base Odoo path verified | Typed DSL obligations and structured API serialization are verified; a real Odoo transaction persists `to approve` and one Activity without rollback. |
| RQ4: ERP empirical evaluation | Partially complete | Seven real Odoo transaction cases plus a two-session retry case pass over mTLS. Comparative measurement and production-path load evidence remain open. |

## 3. Project 2 integration assessment

The Project 2 addon exists at the path above, but it is a **legacy integration baseline**, not a compatible release artifact. Its migrated replacement is now versioned here and passes seven real transaction cases plus two-session serialization retry. That result does not verify the legacy addon, mTLS, multi-replica behavior or production readiness.

| Interface area | Project 2 baseline | Current PDP contract | Migration requirement |
|---|---|---|---|
| Client transport | `grpc.insecure_channel` | Production configuration requires TLS/mTLS | Build a secure channel from centralized Odoo configuration and verify certificate handling. |
| Caller identity | No bearer metadata | Mandatory JWT bearer token with tenant, subject, issuer, audience and expiry claims | Mint/obtain a service credential at the PEP trust boundary; never trust an Odoo-provided subject as principal identity. |
| Delegation proof | Short HMAC over grant, delegator, agent, amount and expiry; fallback secret in source | Versioned length-prefixed HMAC over the full decision tuple; no default production secret | Generate the canonical current proof at a trusted signer and remove the fallback secret. |
| Generated protobuf | Stale, locally copied generated files | Buf-pinned standard Go/Python clients from `proto/v1/policy.proto` | Replace vendored output with the pinned generated Python client and add a compatibility test. |
| Decision response | String obligations and `advice` map | Structured obligations; no `advice` map guarantee | Map typed obligation `type` and payload to the Odoo approval/activity workflow. |
| Revocation | Best-effort RPC after changing Odoo state | Authenticated, tenant-bound revoke; process-local revocation until durable work completes | Handle RPC failure explicitly and do not claim cluster-wide restart-safe revocation. |

## 4. Claim discipline for the thesis

The following wording is required until new experiments are completed:

| Do not claim | Use instead |
|---|---|
| “27.12 ns”, “36.8M RPS”, “286.3 ns prompt-injection block”, or “0.31 ms Odoo E2E” | “The current local evaluator benchmark is sub-microsecond for its measured in-memory cases; final performance numbers will be reported from a versioned reproducibility run.” |
| “Odoo integration is complete” or “all Odoo security is proven” | “The repository-owned Odoo PEP passes seven real mTLS transaction cases plus two-session retry; multi-replica revocation and release evidence remain open.” |
| “WORM/AES-GCM audit trail is implemented” | “The PDP has redacted bounded PostgreSQL audit delivery. Encryption, spill/replay, tamper evidence and retention guarantees remain future work.” |
| “instant cluster-wide revocation” | “Tenant-scoped local revocation is implemented; durable multi-replica propagation is a release gate.” |
| “zero linear scans” | “The engine uses indexes and COW snapshots, but candidate policy iteration remains a measured limitation.” |

## 5. Completion sequence for the thesis

1. Define replay/idempotency semantics and a delegation key-ring lifecycle; implement proof replay protection.
2. Implement durable revocation propagation and audit encryption/spill/replay.
3. Preserve the verified allow, deny, approval, replay, concurrency, outage, revoke and tamper gate in CI.
4. Verify the same boundary with mTLS; the clean database/image build already passes.
5. Publish versioned functional, security and performance results; then update the thesis with only reproducible measured values.

## 6. Authority order

When documents conflict, use this order: current source and generated protocol, `CURRENT_STATE_AUDIT.md`, this alignment, then the proposal and historical Project 2 documents. Historical documents remain valuable as design intent, but they are not evidence of completion.
