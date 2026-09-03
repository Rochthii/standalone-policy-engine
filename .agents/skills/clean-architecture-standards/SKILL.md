---
name: clean-architecture-standards
description: Strict guidelines for modularization, file size budgets (<= 250 lines), zero hardcoding, and SRP.
---

# Clean Architecture & Production Standards Skill

## 🎯 Mission
Enforce clean architecture standards, eliminate technical debt, prevent monolithic files, and guarantee long-term maintainability.

## 🔑 Critical Engineering Invariants
1. **File Size Budget ($\le 250$ Lines)**:
   - No single source code file should exceed **250 lines**.
   - Single Responsibility Principle (SRP): Each file must have exactly one reason to change.

2. **Zero Hardcoding & Centralized Configuration**:
   - Never hardcode ports, secrets, fallback credentials, or buffer sizes in business logic.
   - All environment variables must be loaded and validated through `internal/config/`.

3. **Domain Constants & Zero Magic Numbers**:
   - All limits and sentinels must be named constants with technical rationale comments (e.g. `MaxASTDepth = 15`).

4. **Layered Separation of Concerns**:
   - **Transport Layer (`internal/server/`)**: Handles serialization, status codes, and auth headers.
   - **Engine Layer (`internal/engine/`)**: Pure in-memory decision evaluation, transport-agnostic, zero-alloc.
   - **Storage Layer (`internal/storage/`)**: Encapsulates PostgreSQL (`pgx`), migrations, and BadgerDB.

5. **Type-Safety & Zero Reflection on Critical Paths**:
   - Never use `interface{}` with `reflect` package on evaluation hot-paths.
   - Use strongly-typed structs, static string maps, and stack scratch buffers.
