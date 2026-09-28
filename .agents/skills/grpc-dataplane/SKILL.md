---
name: grpc-dataplane
description: Implement or review the generated-Protobuf gRPC PDP boundary, mandatory JWT/tenant identity binding, mTLS transport, deadlines, revocation, and versioned delegation routing for the repository Odoo client. Use for internal/server, proto-client compatibility, or PDP client changes; not for policy semantics, canonical-intent design, or deployment architecture.
---

# gRPC Data Plane Boundary

## Establish the evidence boundary

- Read [`CURRENT_STATE_AUDIT.md`](../../../docs/technical-spec/CURRENT_STATE_AUDIT.md) and the active task before changing this boundary.
- Treat [`policy.proto`](../../../proto/v1/policy.proto) as the canonical wire contract. Go and Python stubs are generated artifacts; change the IDL and regenerate them rather than hand-editing generated code.
- The current protected Odoo route still uses delegation proof V1. CBI/proof V2 exists as protocol code but is not route evidence until V2-PROOF-04 passes boundary tests.

## Preserve actual ownership and order

- The unary interceptor in `internal/server/grpc_transport.go` owns trace/correlation context only. Authentication, trusted identity binding, revocation, and delegation validation currently run in `CheckAccess` and its helpers; do not describe or relocate them as interceptor controls without corresponding tests.
- Preserve this fail-closed order: bounded server context; mandatory JWT and tenant equality; signed-claim subject/principal binding; revocation readiness and lookup for delegated calls; versioned proof validation; engine evaluation; typed response, metrics, and audit.
- Never trust request-supplied subject or principal attributes when signed claims provide them. Missing bearer credentials, JWT subject, tenant, or cross-tenant equality must fail before evaluation.
- A delegated request must not reach the engine when revocation state is unavailable, the grant is revoked, or proof validation fails.

## Version and transport rules

- For the high-impact purchase-order route, accept only the explicitly selected proof version. After V2-PROOF-04, require proof V2 bound to the exact CBI and reject missing, unknown, or V1 downgrade attempts before evaluation. Keep V1 only for a named, tested legacy boundary.
- Use standard generated Protobuf gRPC. Do not introduce a custom JSON codec or a second handwritten contract.
- Production requires mutually authenticated TLS on both server and Odoo client. Insecure transport is non-production-only and must remain explicit.
- Keep deadlines configurable and preserve their meaning. Current defaults are approximately 100 ms for server evaluation and 350 ms for the Odoo call; they are budgets, not latency claims.
- Preserve structured distinctions among invalid input, unauthenticated identity, cross-tenant denial, revoked delegation, unavailable revocation state, and deadline/cancellation. The Odoo client must convert transport/security failure to a fail-closed business error, never ALLOW.
- Keep the repository client in `custom_addons/pdp_authorizer/models/pdp_client.py` PID-aware and generated-stub based.

## Validate only the changed boundary

- Server auth/routing change: run focused `internal/server` tests for missing JWT, missing subject, tenant mismatch, trusted identity overwrite, revocation unavailable/revoked, proof-version confusion, and deadline cancellation.
- Contract change: regenerate both languages, require a clean generated diff, and exercise the generated Python client against a live Go server.
- Odoo protected-route change: run focused Python protocol tests plus repository-local Odoo/mTLS E2E cases that assert no unauthorized persistent mutation.
- Map high-impact V2 tests to [`EVALUATION_MATRIX.md`](../../../docs/technical-spec/EVALUATION_MATRIX.md), including downgrade and changed-intent negatives. Do not mark the route verified from unit tests alone.
- Finish with `git diff --check` and record the exact evidence boundary in the task board/current-state audit.
