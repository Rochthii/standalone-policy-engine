# Standalone PBAC/ABAC Policy Engine - AI Coding Rules

## 🎯 Project Context
High-performance Standalone Policy Decision Point (PDP) written in Go.
- **Goal**: Evaluate dynamic attribute-based access control (ABAC) and policy-based access control (PBAC) under massive load (>10,000 RPS) with <1ms latency.
- **ZTA Overlay**: Operates as an OpenZiti Dark Service (zero open inbound TCP ports in production) binding to Ziti overlay.
- **Architecture**: Receives gRPC requests from Policy Enforcement Points (PEP) -> Performs in-memory decision tree matching (Trie, Role DAG) -> Evaluates AST conditions -> Returns ALLOW/DENY.

---

## 🛠️ AI Coding Rules

### 1. High Performance & Low Allocation
- **No Linear Scan**: Never loop over the entire list of policies. Organize policy lookup using Trie structure based on `Subject` and `Resource` to achieve $O(1)$ or $O(\log N)$ matching complexity.
- **Minimize Memory Allocations**: Tận dụng `sync.Pool` for evaluation contexts, token buffers, or AST evaluation contexts to reduce Garbage Collector (GC) pressure.
- **Concurrency & Lock Optimization**: Use `sync.RWMutex` for read-heavy operations. Rely on lock-free reads or double-checking locks when updating RAM stores to avoid thread blocking.

### 2. AST Evaluator & Policy Language
- **No Hardcoded Rules**: Business policy logic must reside in database/policies configurations and parsed into Abstract Syntax Trees (AST).
- **AST Safety**: Sanitize and evaluate AST nodes safely to prevent Arbitrary Code Execution (ACE) during dynamic ABAC condition checks.

### 3. gRPC & JSON-over-gRPC Codec
- **JSON gRPC Codec**: Enforce custom JSON codec (`jsonCodec`) registered under name `"json"`. Do not fallback to standard protobuf binary serialization.
- **No JSON omitempty on Enums**: The `decision` field in gRPC check access structures must **never** contain `omitempty` tags. This ensures `DENY (0)` values are serialized.
- **OpenZiti SDK Listener**: Support embedded OpenZiti listener (`zCtx.Listen()`) when `USE_ZITI=true` to enable dark service mode.
- **Keep-Alive Configuration**: Set aggressive keep-alive parameters (e.g. max connection age, client pings) to optimize persistent TCP/Ziti overlay channels.

---

## 📂 Key File Map
- **Server Entry:** [main.go](file:///e:/Projects/Project_TN/standalone-policy-engine/cmd/pdp-server/main.go)
- **gRPC Server Handler & Listener:** [grpc_server.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/server/grpc_server.go)
- **In-Memory Store & Trie:** [store.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/store.go)
- **AST Parser & Evaluator:** [evaluator.go](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go)
- **Protobuf/gRPC Service Definition:** [policy.proto](file:///e:/Projects/Project_TN/standalone-policy-engine/proto/v1/policy.proto)
