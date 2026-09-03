# PROTOCOL_CONTRACT.md — Interface & Data Contracts

## 1. Protobuf Service Specification

```protobuf
syntax = "proto3";

package policy.v1;

option go_package = "github.com/Rochthii/standalone-policy-engine/proto/v1;policyv1";

service PolicyDecisionPoint {
  // Synchronous, low-latency access evaluation (< 0.35ms E2E)
  rpc CheckAccess (CheckAccessRequest) returns (CheckAccessResponse);
  
  // Audited access decision with full evaluation trace
  rpc ExplainDecision (CheckAccessRequest) returns (ExplainDecisionResponse);
  
  // Instant delegation revocation (TOCTOU mitigation < 1 µs)
  rpc RevokeDelegation (RevokeRequest) returns (RevokeResponse);
}

enum Decision {
  DECISION_UNSPECIFIED = 0;
  ALLOW = 1;
  DENY = 2;
}

message CheckAccessRequest {
  string tenant_id = 1;               // e.g. "my_company"
  string subject   = 2;               // e.g. "agent:procurement_copilot" or "user:alice"
  string action    = 3;               // e.g. "action:APPROVE_PURCHASE_ORDER"
  string resource  = 4;               // e.g. "purchase_order:PO00042"
  map<string, string> context = 5;    // Dynamic evaluation attributes
}

message CheckAccessResponse {
  Decision decision = 1;              // ALLOW or DENY
  string matched_policy_id = 2;       // ID of the decisive permit/forbid rule
  repeated string obligations = 3;    // e.g. ["REQUIRE_HUMAN_APPROVAL"]
  map<string, string> advice = 4;     // Supplemental metadata for PEP
  int64 evaluation_nanos = 5;         // In-memory evaluation duration (e.g. 27)
}

message RevokeRequest {
  string tenant_id = 1;               // e.g. "my_company"
  string grant_id  = 2;               // Delegation grant ID (e.g. "42")
  string revoked_by = 3;              // e.g. "user:manager_bob"
  string reason    = 4;               // e.g. "User initiated revocation from Odoo UI"
}

message RevokeResponse {
  bool success     = 1;
  int64 revoked_at = 2;               // Unix timestamp of revocation in RAM
  string message   = 3;
}


message ExplainDecisionResponse {
  Decision decision = 1;
  string matched_policy_id = 2;
  repeated string explanations = 3;
  repeated string obligations = 4;
  map<string, string> advice = 5;
}
```

---

## 2. Scope Resolution Specification (Prefixed Scope Convention)

Engine resolution implementation ([`internal/engine/evaluator.go:101-155`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go#L101-L155)):

| DSL Scope | Target Field in DSL | Primary Key in Request `context` | Fallback Key in Request `context` | Resolution Logic in Go Engine |
|---|---|---|---|---|
| `principal.*` | `principal.id` | N/A (Derived from `req.Subject`) | N/A | Direct string binding from `req.Subject` |
| `principal.*` | `principal.role` | `principal.role` | `role` | Checks `ctx.Context["principal.role"]`, then `ctx.Context["role"]` |
| `principal.*` | `principal.department` | `principal.department` | `department` | Checks `ctx.Context["principal.department"]`, then `ctx.Context["department"]` |
| `resource.*` | `resource.id` | N/A (Derived from `req.Resource`) | N/A | Direct string binding from `req.Resource` |
| `resource.*` | `resource.creator_id` | `resource.creator_id` | `creator_id` | Checks `ctx.Context["resource.creator_id"]`, then `ctx.Context["creator_id"]` |
| `resource.*` | `resource.status` | `resource.status` | `status` | Checks `ctx.Context["resource.status"]`, then `ctx.Context["status"]` |
| `action.*` | `action.id` | N/A (Derived from `req.Action`) | N/A | Direct string binding from `req.Action` |
| `context.*` | `context.amount` | `amount` | `context.amount` | Checks `ctx.Context["amount"]`, then `ctx.Context["context.amount"]` |
| `context.*` | `context.delegation_chain` | `delegation_chain` | `context.delegation_chain` | Checks `ctx.Context["delegation_chain"]`, then `ctx.Context["context.delegation_chain"]` |
| `context.*` | `context.delegation_proof` | `delegation_proof` | `context.delegation_proof` | Evaluated at gRPC Security Interceptor (Layer 1) |

---

## 3. Data Serialization Conventions (Invariants)

1. **Raw Integer Monetary Values**:
   - MUST serialize currency values as raw integer strings: `"1500"`, `"50000"`.
   - FORBIDDEN: UI formatting strings containing commas or dots (`"50.000.000"` or `"50,000.00"`).
   - *Rationale*: `strconv.ParseInt` fails silently on punctuation, causing `amount > 2000` to evaluate to `false` and bypassing `forbid` rules.
2. **Comma-Separated Delegation Chain**:
   - MUST format delegation paths as a flat comma-separated string without extraneous whitespace:
     ```python
     f"user:{order.delegated_by_id.login},agent:{order.ai_agent_id}"
     # Output: "user:manager_bob,agent:procurement_copilot"
     ```
   - *Rationale*: Matches `evaluator.go:387` (`BinOpContains`) array membership scanning.

---

## 4. Production Payload Contracts

### Case 1: AI Agent Within Autonomous Limit ($1,500 <= $2,000) -> ALLOW

**Odoo PEP Request:**
```json
{
  "tenant_id": "my_company",
  "subject": "agent:procurement_copilot",
  "action": "action:APPROVE_PURCHASE_ORDER",
  "resource": "purchase_order:PO00042",
  "context": {
    "amount": "1500",
    "currency": "USD",
    "resource.creator_id": "user:alice",
    "is_ai_generated": "true",
    "delegation_chain": "user:manager_bob,agent:procurement_copilot",
    "delegation_proof": "a9f3b17c8d9e204859a0f448b30174e2d354e1951048b29e0721da12847c94b3",
    "tool_context": "tool:auto_confirm_po",
    "execution_mode": "autonomous_run"
  }
}
```

**Go PDP Response:**
```json
{
  "decision": "ALLOW",
  "matched_policy_id": "POL-AI-AUTONOMOUS-PERMIT-01",
  "obligations": [],
  "advice": {
    "risk_level": "LOW"
  },
  "evaluation_nanos": 27
}
```

---

### Case 2: AI Agent Exceeds Threshold ($50,000 > $2,000) -> DENY + REQUIRE_HUMAN_APPROVAL

**Odoo PEP Request:**
```json
{
  "tenant_id": "my_company",
  "subject": "agent:procurement_copilot",
  "action": "action:APPROVE_PURCHASE_ORDER",
  "resource": "purchase_order:PO00099",
  "context": {
    "amount": "50000",
    "currency": "USD",
    "resource.creator_id": "user:alice",
    "is_ai_generated": "true",
    "delegation_chain": "user:manager_bob,agent:procurement_copilot",
    "delegation_proof": "7c1e9b4d8a1f203849c0e558a20184d3c243f1841037a18d0610ea11736b83a2",
    "tool_context": "tool:auto_confirm_po",
    "execution_mode": "autonomous_run"
  }
}
```

**Go PDP Response:**
```json
{
  "decision": "DENY",
  "matched_policy_id": "POL-AI-AUTONOMOUS-FORBID-02",
  "obligations": [
    "REQUIRE_HUMAN_APPROVAL"
  ],
  "advice": {
    "risk_level": "HIGH",
    "required_approver_role": "role:finance_director",
    "reason": "Amount exceeds AI autonomous ceiling ($2,000). Routed to supervisor approval."
  },
  "evaluation_nanos": 31
}
```
