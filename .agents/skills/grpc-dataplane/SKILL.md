---
name: grpc-dataplane
description: Expert rules for gRPC Data Plane, Layer 1 Interceptors, Revocation RPC, and PID-safe Odoo client.
---

# gRPC Data Plane Server Skill

## 🎯 Mission
Serve sub-microsecond `CheckAccess`, `ExplainDecision`, and `RevokeDelegation` with multi-tenant isolation and fail-closed security.

## 🔑 Critical Implementation Rules
1. **gRPC Protocol & Codec**:
   - Custom JSON codec `"json"` via `grpc.CallContentSubtype("json")`.
   - Never use `omitempty` on `Decision` enum (`DENY = 0`, `ALLOW = 1`).
   - RPCs:
     - `CheckAccess(CheckAccessRequest) returns (CheckAccessResponse)`
     - `ExplainDecision(ExplainRequest) returns (ExplainResponse)`
     - `RevokeDelegation(RevokeRequest) returns (RevokeResponse)`

2. **Layer 1 Security Interceptors (Pre-Engine Check)**:
   - **Revocation Check**: If `req.Context["delegation_grant_id"]` is in `revocationMap` $\to$ return `Decision_DENY` (`POL-REVOCATION-BLACK-LIST`) in $< 50$ns without hitting engine.
   - **HMAC Proof Verification**: If `req.Context["delegation_proof"]` is present $\to$ verify Canonical String and TTL expiration. If invalid/expired $\to$ return `codes.PermissionDenied` (403).
   - **Tenant Isolation**: Strictly enforce `claims["tenant_id"] == req.TenantId`.

3. **PID-Safe gRPC Client (Odoo PEP)**:
   - File: `custom_addons/pdp_authorizer/models/pdp_client.py`.
   - Track `self._pid = os.getpid()`. When Odoo forks a worker, recreate channel if `os.getpid() != self._pid` to prevent C-Core epoll deadlock.
   - Fail-Closed: Network/PDP outage raises `AccessError` (never silently allows).

4. **Keep-Alive & Tuning**:
   - Client timeout: 350ms per request.
   - Server KeepAlive: `MinTime: 5s`, `PermitWithoutStream: true`, `MaxConnectionIdle: 15s`.

## 📂 Source Files
- [`proto/v1/policy.proto`](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto)
- [`internal/server/grpc_server.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/grpc_server.go)
- [`custom_addons/pdp_authorizer/models/pdp_client.py`](file:///E:/Projects/ERP_Mastery_Hub/02_Project_2_Odoo_Go_PDP_Approval/custom_addons/pdp_authorizer/models/pdp_client.py)
