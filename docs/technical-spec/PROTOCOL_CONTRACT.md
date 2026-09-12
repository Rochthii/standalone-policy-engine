# Protocol and Data Contract

> **Updated:** 2026-09-12
>
> **Normative source:** [`proto/v1/policy.proto`](../../proto/v1/policy.proto). Generated files are outputs, never hand-edited.

## 1. Generation and wire protocol

The service uses standard Protocol Buffers over gRPC. The former handwritten Go structs, custom JSON content subtype and unused `proto/policy.proto` v0 contract have been removed.

```bash
# Generate Go and Python clients with pinned Buf/plugin versions.
make generate-proto

# Lint, format-check, regenerate and reject checked-in drift.
make proto-check
```

Generated clients:

- Go: `proto/v1/policy.pb.go`, `proto/v1/policy_grpc.pb.go`
- Python: `clients/python/v1/policy_pb2.py`, `clients/python/v1/policy_pb2_grpc.py`
- Python runtimes: `protobuf==6.32.1`, `grpcio==1.75.1`

## 2. RPC boundary

| RPC | Authentication | Authorization / isolation | Main status codes |
|---|---|---|---|
| `CheckAccess` | Exactly one Bearer JWT | JWT tenant must equal request tenant; JWT subject and principal attributes are authoritative | `Unauthenticated`, `PermissionDenied`, `InvalidArgument`, `DeadlineExceeded`, `ResourceExhausted` |
| `ExplainDecision` | Exactly one Bearer JWT | Same tenant/identity boundary as `CheckAccess` | Same authentication/limit codes |
| `RevokeDelegation` | Exactly one Bearer JWT | JWT tenant match, `revoked_by` match and `delegation:revoke` permission | `Unauthenticated`, `PermissionDenied`, `InvalidArgument` |

Runtime limits are fail-fast configuration:

| Environment key | Default | Meaning |
|---|---:|---|
| `GRPC_EVALUATION_TIMEOUT` | `100ms` | Maximum server-side `CheckAccess` evaluation time; a shorter client deadline still wins |
| `GRPC_MAX_RECEIVE_BYTES` | `1048576` | Maximum inbound protobuf message size |
| `GRPC_MAX_SEND_BYTES` | `1048576` | Maximum outbound protobuf message size |

## 3. CheckAccess messages

```protobuf
message CheckAccessRequest {
  string tenant_id = 1;
  string subject = 2;
  string action = 3;
  string resource = 4;
  map<string, string> context = 5;
}

message Obligation {
  string type = 1;
  string message = 2;
  map<string, string> payload = 3;
}

message CheckAccessResponse {
  enum Decision {
    DENY = 0;
    ALLOW = 1;
  }
  Decision decision = 1;
  string matched_policy_id = 2;
  repeated Obligation obligations = 3;
  map<string, string> advice = 4;
}
```

`subject` remains on the wire for compatibility, but the server overwrites it with JWT `sub`. Callers cannot use it to select another principal.

## 4. Trusted attribute resolution

| DSL namespace | Resolution |
|---|---|
| `principal.id` | JWT `sub` |
| `principal.*` | Signed JWT principal attributes copied to `principal.<name>`; request values are overwritten |
| `resource.id` | `request.resource` |
| `resource.*` | Exact namespaced request key such as `resource.creator_id`; no raw-key fallback |
| `action.id` | `request.action` |
| `context.*` | Dynamic request context, resolved as the raw field name then `context.<name>` |

Missing attributes fail closed during condition evaluation.

## 5. Delegation proof tuple

A delegated call is any request containing `delegation_grant_id`. Such a call must include a versioned HMAC proof over the complete length-prefixed tuple:

`tenant_id`, `grant_id`, `delegator`, `agent`, `action`, `resource`, `amount`, `delegation_chain`, `resource.creator_id`, `tool_context`, `execution_mode`, `delegation_nonce`, `delegation_issued_at`, `delegation_valid_until`.

The proof envelope is `v1.<kid>.<hex-hmac>`. The key ID is included in the canonical HMAC bytes and selects an explicitly configured verification key. New proofs use `PDP_DELEGATION_ACTIVE_KID`; `PDP_DELEGATION_KEYS_JSON` retains active and grace-period keys during rotation. Production must use the explicit key ring. `PDP_SHARED_SECRET` remains a development/test compatibility fallback only.

The maximum validity window is 24 hours. The chain must be the direct `delegator,agent` pair. `CheckAccess` is idempotent and does not consume the nonce; exact business-action replay safety belongs to the Odoo transaction ledger described in [`DELEGATION_REPLAY_IDEMPOTENCY.md`](./DELEGATION_REPLAY_IDEMPOTENCY.md).

## 6. Serialization invariants

- Monetary values are base-10 integer strings such as `"1500"`; localized punctuation is invalid.
- Delegation timestamps are Unix seconds encoded as decimal strings.
- Delegation chains are comma-separated identities without whitespace.
- Proof, nonce, authorization, token, secret and configured PII context keys are redacted before entering the audit queue.
- Policy actions may include or omit the `action:` prefix; the engine normalizes it internally.

## 7. Structured obligation example

Policy:

```cedar
forbid(
  principal == agent:procurement_copilot,
  action == action:APPROVE_PURCHASE_ORDER,
  resource == any
)
when { context.amount > 2000 }
obligation REQUIRE_HUMAN_APPROVAL "Autonomous agent spending requires human approval";
```

Response:

```json
{
  "decision": "DENY",
  "matched_policy_id": "POL-AI-AUTONOMOUS-FORBID-02",
  "obligations": [
    {
      "type": "REQUIRE_HUMAN_APPROVAL",
      "message": "Autonomous agent spending requires human approval"
    }
  ]
}
```

The PEP must treat unknown obligation types as non-executable and fail closed. The current compiler accepts only `REQUIRE_HUMAN_APPROVAL`, `AUDIT_SENSITIVE_TOOL_CALL`, and `MASK_ATTRIBUTES`.

## 8. Evidence

- Standard Go protobuf client crossed Docker TCP to the production PDP server and passed ALLOW, DENY and policy catch-up cases.
- Generated Python client imported with the pinned runtimes, serialized structured obligations, and called a live Go gRPC server successfully.
- An oversized protobuf request is rejected with `ResourceExhausted` by an automated wire-level test.
- The repository-owned Odoo 17 addon used the generated Python client across the live Docker gRPC boundary and passed seven real ORM/PostgreSQL transaction cases plus a two-session serialization-retry assertion.

The Odoo evidence covers hostname-verified mutual TLS, missing-client-certificate rejection, duplicate/altered-command replay, rollback, non-rollback and two-session nonce concurrency. It remains single-PDP evidence and does not prove multi-replica revocation or release readiness.
