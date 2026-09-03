---
name: agent-authorization
description: Expert rules for Unified AI Agent Authorization, Delegation Chains, HMAC Canonical String, and Deterministic Guardrails.
---

# AI Agent Authorization & Guardrails Skill

## 🎯 Mission
Provide deterministic authorization guardrails for Autonomous AI Agents calling ERP tools, mitigating OWASP LLM06 and prompt injection in $< 600$ns with Zero GC allocations.

## 🔑 Critical Invariants & Rules
1. **Delegation Tuple ($\Delta$)**:
   $\Delta = \langle \mathcal{U}_{\text{root}}, \mathcal{A}_{\text{exec}}, \Sigma_{\text{scope}}, \Omega_{\text{constraints}}, \mathcal{C}_{\text{chain}} \rangle$
   - $\mathcal{U}_{\text{root}}$: Human delegator (e.g. `user:bob`).
   - $\mathcal{A}_{\text{exec}}$: Executing agent (e.g. `agent:procurement_copilot`).
   - $\Omega_{\text{constraints}}$: Autonomous spending ceiling ($\le \$2,000$, TTL, `tool:auto_confirm_po`).
   - $\mathcal{C}_{\text{chain}}$: Delegation trace (`user:bob,agent:procurement_copilot`). Depth = 1.

2. **HMAC Canonical String & TTL Check**:
   $\text{Payload} = \text{grant\_id} \parallel \text{delegator} \parallel \text{agent} \parallel \text{amount} \parallel \text{valid\_until}$
   - Formula: `fmt.Sprintf("%s|%s|%s|%s|%s", grantID, delegator, agent, amount, validUntil)`
   - TTL Expiration: `time.Now().Unix() > expTimestamp` evaluates to `false` (Fail-Closed, 403).
   - Tampered payload or mismatch signature fails via `hmac.Equal` (403 PermissionDenied).

3. **In-Memory RevocationMap $O(1)$ (Anti-TOCTOU)**:
   - Managed via `sync.Map` in `internal/security/delegation.go`.
   - RPC `RevokeDelegation` stores `grant_id -> revoked_at` in RAM in $< 50$ns.
   - Interceptor rejects subsequent requests immediately with `Decision_DENY` (`POL-REVOCATION-BLACK-LIST`).

4. **Generalized SoD Preservation**:
   - `context.delegation_chain contains resource.creator_id` triggers `forbid` via `evaluator.go`.
   - Neither the creator nor delegator can self-approve, directly or via delegated AI agents.

5. **Non-Rollback PEP Pattern (Odoo 17)**:
   - When decision is `DENY` with obligation `REQUIRE_HUMAN_APPROVAL`, Odoo PEP writes PO `state = 'to approve'`, schedules Activity, and returns `True` (zero DB rollback).

## 🧪 7 E2E Vectors ([`tests/e2e_delegation_test.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/tests/e2e_delegation_test.go))
- `TC-01`: Self-approval $\to$ DENY (SoD).
- `TC-02`: Agent approves Delegator's PO $\to$ DENY (SoD Chain).
- `TC-03`: Agent autonomous PO $\le \$2,000 \to$ ALLOW.
- `TC-04`: Agent PO $> \$2,000 \to$ DENY (`REQUIRE_HUMAN_APPROVAL`).
- `TC-05`: Tampered HMAC amount $\to$ 403 PermissionDenied.
- `TC-06`: Revoked Grant on RAM $\to$ DENY (Anti-TOCTOU).
- `TC-07`: Expired TTL Proof $\to$ 403 PermissionDenied.
