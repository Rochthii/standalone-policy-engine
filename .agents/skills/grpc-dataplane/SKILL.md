---
name: grpc-dataplane
description: Expert rules and constraints for gRPC Data Plane, JSON-over-gRPC, mTLS, Keep-Alive, and OpenZiti Dark Service.
---

# gRPC Data Plane Server Skill

## 🎯 Mission
Serve ultra-fast `CheckAccess` and `ExplainDecision` requests with multi-tenant security and zero trust network transport.

## 🔑 Critical Implementation Rules
1. **gRPC Protocol & Codec**:
   - Registered custom JSON codec: `"json"` via `grpc.CallContentSubtype("json")`.
   - Never use `omitempty` on `Decision` enum (`DENY = 0`, `ALLOW = 1`).
   - Timeout: Enforce 100ms per-request timeout context to prevent thread starvation.
2. **Multi-Tenant JWT Isolation**:
   - Extract `claims["tenant_id"]` from `Authorization: Bearer <token>`.
   - Strictly reject request if `claims["tenant_id"] != req.TenantId` with `codes.PermissionDenied`.
   - Enrich `req.Subject` and context attributes from validated JWT claims.
3. **Transport Security (mTLS & OpenZiti)**:
   - **mTLS**: When `PDP_TLS_CERT`, `PDP_TLS_KEY`, `PDP_TLS_CA` are provided, require `tls.RequireAndVerifyClientCert` with TLS 1.2+ minimum.
   - **OpenZiti Dark Service**: When `USE_ZITI=true`, bind exclusively via `zCtx.Listen(serviceName)` with zero open inbound TCP ports.
4. **Persistent Connection Tuning**:
   - KeepAlive Enforcement: `MinTime: 5s`, `PermitWithoutStream: true`.
   - KeepAlive Server: `MaxConnectionIdle: 15s`, `MaxConnectionAge: 30m`, `Time: 5s`, `Timeout: 1s`.
   - Tracing: Intercept `x-trace-id` / W3C `traceparent` metadata.

## 📂 Source Files
- [`internal/server/grpc_server.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/grpc_server.go)
- [`cmd/pdp-server/main.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go)
- [`internal/security/jwt.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/security/jwt.go)
- [`proto/v1/policy.proto`](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto)
