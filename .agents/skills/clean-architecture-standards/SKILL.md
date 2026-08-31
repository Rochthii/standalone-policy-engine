---
name: clean-architecture-standards
description: Strict guidelines for modularization, file size budgets (<= 200 lines), zero hardcoding, centralized type-safe config, and enterprise-grade clean architecture.
---

# Clean Architecture & Production Engineering Standards Skill

## 🎯 Mission
Enforce international clean architecture standards, eliminate technical debt, prevent monolithic files, ban hardcoded parameters, and guarantee long-term maintainability.

---

## 🔑 Critical Engineering Invariants

### 1. 📏 File Size Budget (Max 150 - 250 Lines)
- **Hard Rule**: No single source code file should exceed **250 lines**.
- **Monolith Splitting**:
  - `http_server.go` $\to$ Split into `http_server.go` (lifecycle/routing), `handlers_policy.go`, `handlers_tenant.go`, `handlers_decision.go`.
  - `postgres.go` $\to$ Split into `postgres.go` (pool/migrations), `policy_repo.go`, `tenant_repo.go`.
- **Single Responsibility Principle (SRP)**: Each file must have exactly one reason to change.

### 2. 🚫 Zero Hardcoding & Centralized Configuration
- **Hard Rule**: Never hardcode ports, connection strings, fallback credentials, timeout durations, or buffer sizes in business logic or handlers.
- **Single Source of Truth**: All environment variables and configurations must be loaded, parsed, and validated through internal/config/.
- **Fail-Fast Validation**: Application must refuse to boot if required secrets or invalid configuration ranges are detected in production mode.

### 3. 🔢 Domain Constants & Zero Magic Numbers
- **Hard Rule**: Never use raw numbers (e.g. 15, 1000, 1024, 5000) directly in algorithms.
- **Dedicated Constants**: All limits must be named constants defined in constants.go with technical rationale comments (e.g. MaxASTDepth = 15, DefaultRingBufferCapacity = 1000).

### 4. 🏛️ Layered Separation of Concerns
- **Transport Layer (HTTP/gRPC)**: Only handles serialization, deserialization, status codes, and auth headers. Never write business calculations or raw SQL in handlers.
- **Engine / Domain Layer**: Pure, transport-agnostic, zero-network logic. Operates solely in memory with strongly-typed structures.
- **Storage / Repository Layer**: Encapsulates all SQL, transactions, and persistence adapters.

### 5. 🛡️ Type-Safety & Zero Reflection on Critical Paths
- **Hard Rule**: Never use map[string]interface{} with eflect package on evaluation or ingestion paths.
- Use strongly-typed structs, static string maps, and stack-allocated byte scratch buffers.

---

## 📂 Enforced Directory Layout
- internal/config/: Type-safe configuration parser and validator.
- internal/server/: Modular HTTP and gRPC transport handlers (< 150 lines each).
- internal/engine/: Core In-Memory Decision Engine and constants.
- internal/storage/: Clean database repositories and migration adapters.
