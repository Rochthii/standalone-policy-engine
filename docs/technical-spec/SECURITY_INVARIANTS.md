# SECURITY_INVARIANTS.md — Mathematical Model & Security Invariants

## 1. Formal Model of Constrained Delegation ($\Delta$)

The delegation of authority from an authenticated human principal to an autonomous AI agent is formally modeled as a 5-tuple:

$$\Delta = \langle \mathcal{U}_{\text{root}}, \mathcal{A}_{\text{exec}}, \Sigma_{\text{scope}}, \Omega_{\text{constraints}}, \mathcal{C}_{\text{chain}} \rangle$$

| Symbol | Element | Definition | Enterprise Example |
|---|---|---|---|
| $\mathcal{U}_{\text{root}}$ | Root Delegator | The authoritative human principal delegating power | `user:manager_bob` |
| $\mathcal{A}_{\text{exec}}$ | Delegatee Agent | The autonomous workload or AI execution identity | `agent:procurement_copilot` |
| $\Sigma_{\text{scope}}$ | Functional Scope | Permitted set of actions, tools, and resource classes | `action:APPROVE_PO` on `resource:purchase_order` |
| $\Omega_{\text{constraints}}$ | Guardrail Boundaries | Operational limits: budget ceilings, TTL, IP whitelists | $\text{amount} \le 2000$, $\text{valid\_until} \le t_{\text{exp}}$ |
| $\mathcal{C}_{\text{chain}}$ | Delegation Lineage | Directed delegation graph: $\mathcal{U}_{\text{root}} \to \mathcal{A}_{\text{exec}}$ ($\text{Depth} = 1$) | `"user:manager_bob,agent:procurement_copilot"` |

---

## 2. Core Security Invariants

### Invariant 1: Time-Aware Monotonic Attenuation
An agent's effective permissions $\mathcal{P}_{\text{effective}}$ at time $t$ can **never** exceed the delegator's active rights, bounded strictly by delegation scope and deterministic guardrails:

$$\mathcal{P}_{\text{effective}}(\mathcal{A} \mid \mathcal{U}, t) = \mathcal{P}_{\text{active}}(\mathcal{U}, t) \cap \mathcal{S}_{\text{delegation}} \cap \Omega_{\text{guardrails}}$$

- **Implemented attenuation (VERIFIED within the stated boundary):** the Odoo PEP requires an active 1-hop grant, and the PDP verifies a tenant-bound, TTL-bounded full tuple before policy evaluation. A revoked grant is denied before the evaluator; policy guardrails and SoD can further deny the call.
- **Dynamic collapse (NOT VERIFIED):** suspension, departure, or a daily approval limit would require a trusted, current source for the delegator's status/limit. The current request builder, PDP tuple and tests do not supply or evaluate `delegator_status` or `delegator_limit`. This remains a future design property, not a thesis result.

---

### Invariant 2: Generalized Separation of Duties (SoD)
The resource creator is prohibited from appearing anywhere within the approval delegation chain:

$$\mathcal{U}_{\text{creator}} \notin \mathcal{C}_{\text{chain}}(\text{Approver})$$

- **Threat Vector**: A malicious user creates a fraudulent purchase order, then triggers an AI Copilot (either their own or an AI acting under delegation from their manager) to approve the order.
- **Engine Enforcement (VERIFIED):** the evaluator's `BinOpContains` applies the policy predicate to the proof-bound `delegation_chain` and `resource.creator_id`. The in-process seven-vector suite covers self-approval and the delegator-created-order chain collision; the Odoo mTLS suite covers the self-approval rollback path:
  ```cedar
  forbid(
      principal == any,
      action    == action:APPROVE_PURCHASE_ORDER,
      resource  == any
  )
  when {
      context.delegation_chain contains resource.creator_id
  };
  ```

---

### Invariant 3: Enforcement Point Integrity & Proof Authenticity
Contextual attributes self-reported across process boundaries must be cryptographically verifiable:

$$\text{Proof} = \text{HMAC-SHA256}\Big(K_{kid}, \; \text{versioned length-prefixed full decision tuple}\Big)$$

The current tuple binds tenant, grant, delegator, agent, action, resource, amount/constraints, delegation chain, creator, tool context, execution mode, nonce and validity window. HMAC provides integrity and authenticity for key holders; it is not non-repudiation.

- **Threat Vector**: A rogue actor crafts arbitrary JSON requests with forged `delegation_chain: "user:cfo_john,agent:copilot"`.
- **Architectural Boundary (VERIFIED):** [`GRPCServer.CheckAccess`](../../internal/server/grpc_server.go) binds the JWT identity, checks revocation readiness/revocation, then calls [`validateDelegation`](../../internal/server/delegation_auth.go) before `Engine.CheckPermission`. This is the implemented Go PDP security boundary; it is not a separate gRPC unary interceptor. The evaluator therefore receives the server-normalized request after the delegated-request checks pass. Latency is reported separately for the measured evaluator, local application path and Odoo boundary.

---

## 3. Real-Time Revocation & TOCTOU Mitigation

**Time-of-Check to Time-of-Use (TOCTOU)** vulnerability occurs if user revocation in ERP takes seconds to propagate while an agent fires automated tool-calls in milliseconds.

```mermaid
sequenceDiagram
    autonumber
    actor Manager as Manager Bob
    participant Odoo as Odoo PEP
    participant Interceptor as PDP Layer 1 (Interceptor)
    participant RAM_Revoke as In-Memory RevocationMap
    participant Agent as Rogue AI Agent

    Manager->>Odoo: Click "Revoke AI Delegation"
    Odoo->>Interceptor: gRPC RevokeDelegation(session_id)
    Interceptor->>RAM_Revoke: Store(session_id, revoked_at) [< 1 µs]
    Interceptor-->>Odoo: Revocation ACK
    
    Note over Agent,Interceptor: Concurrently, Agent fires approval tool-call:
    Agent->>Interceptor: gRPC CheckAccess(session_id)
    Interceptor->>RAM_Revoke: Load(session_id) -> Exists!
    Interceptor-->>Agent: Immediate fail-closed DENY; propagation and latency use recorded boundary evidence
```

- **Data Structure (VERIFIED):** a tenant-scoped `sync.Map` holds active revocations with TTL for the process-local lookup.
- **Boundary:** PostgreSQL snapshot-first propagation, delayed delivery and restart are verified only by the local three-replica integration test (108 samples; recorded maximum 38.8256 ms against the 5 s test SLO). The Odoo suite proves a live revoke against one PDP, not cluster behavior. Delegated checks fail closed while synchronization is unavailable. This is neither a universal propagation guarantee nor production evidence.
