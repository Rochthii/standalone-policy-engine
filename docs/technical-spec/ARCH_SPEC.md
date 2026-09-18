# ARCH_SPEC.md — System Architecture & Responsibility Boundaries

## 1. End-to-End Data Flow Architecture

```
[Odoo 17 Enterprise ERP]
  │
  ├─ User / AI Copilot invokes button_confirm()
  │
  ▼
[Odoo PEP Hook: custom_addons/pdp_authorizer]
  │
  ├─ 1. Extract ORM attributes: creator_id, amount_untaxed, company_id
  ├─ 2. Resolve delegation session & compute HMAC-SHA256 delegation_proof
  ├─ 3. Format payload: raw integer amount, comma-separated delegation_chain
  │
  ▼ gRPC Persistent Connection (HTTP/2 mTLS :50051)
┌─────────────────────────────────────────────────────────────────────────────┐
│ PDP GATEWAY: gRPC Security Interceptor (Tầng 1 - Border Security)          │
├─────────────────────────────────────────────────────────────────────────────┤
│ 1. Mutual TLS (mTLS) X.509 Certificate Validation                           │
│ 2. Tenant Isolation Enforcement: claims["tenant_id"] == req.TenantId        │
│ 3. Cryptographic Verification: versioned full-tuple HMAC delegation proof   │
│ 4. TOCTOU Mitigation: process-local O(1) lookup plus PostgreSQL propagation  │
│    └─ If revoked -> Short-circuit Hard DENY                                 │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │ Verified Clean Request Context
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PDP CORE: In-Memory Decision Engine (latency reported per measured case)     │
├─────────────────────────────────────────────────────────────────────────────┤
│ 1. Radix Trie Policy Filtering (FNV-1a 64-bit uint64 index):               │
│    TenantID -> SubjectHash -> ResourceHash -> ActionHash                    │
│ 2. Role DAG Transitive Closure Query: O(1) IsDescendant()                   │
│ 3. AST Pure Evaluation: sync.Pool EvalContext, Stack Scratch [64],          │
│    0 allocs/op for measured evaluator cases; bytes vary by workload          │
│ 4. Decision Synthesis: Deny-by-Default + Forbid-Overrides                   │
│ 5. Obligation Mapping: Matched Policy -> Obligations/Advice                 │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │ Decision Tuple: <Decision, Obligations>
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ AUDIT PIPELINE: Async Non-Blocking Telemetry                               │
├─────────────────────────────────────────────────────────────────────────────┤
│ Bounded async queue -> PostgreSQL encrypted storage                          │
│ (external append-only retention/deletion evidence is deferred until 2029)   │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼ gRPC response (no production E2E latency SLO established)
[Odoo PEP Hook: State Coordinator]
  │
  ├─ ALLOW: order.pdp_status = 'allow' -> super().button_confirm() (Workflow continues)
  ├─ DENY + REQUIRE_HUMAN_APPROVAL:
  │    order.write({'state': 'to approve', 'pdp_status': 'require_approval'})
  │    order.activity_schedule(...) [NO EXCEPTION, NO DB ROLLBACK]
  └─ Hard DENY / SoD Violation:
       order.write({'pdp_status': 'deny'}) -> raise AccessError(...) [ROLLBACK]
```

---

## 2. Component Responsibility Matrix

| Component | Physical Location | Runtime / Language | Primary Responsibilities | Non-Responsibilities |
|---|---|---|---|---|
| **Odoo PEP Module** | `custom_addons/pdp_authorizer/` | Python 3.10+ / Odoo ORM | • Intercept `button_confirm`<br>• PIP: Extract ORM attributes (`resource.creator_id`)<br>• Sign `delegation_proof`<br>• Coordinate ORM state transitions without rollback | • Policy logic execution<br>• In-memory rule storage<br>• Token parsing |
| **gRPC Security Interceptor** | `internal/security/` | Go / gRPC Server | • mTLS X.509 handshake<br>• JWT claims validation<br>• Full-tuple HMAC proof verification<br>• In-Memory `RevocationMap` $O(1)$ query | • AST policy evaluation<br>• Radix Trie indexing<br>• Disk I/O |
| **In-Memory Decision Engine** | `internal/engine/` | Go | • Multi-level Radix Trie index<br>• Precomputed Role DAG closure $O(1)$<br>• Pure AST evaluation with measured-case allocation results<br>• Decision synthesis & Obligation attachment | • Cryptographic hashing<br>• Database queries<br>• Network protocol handling |
| **State Synchronizer** | `internal/storage/`, `internal/engine/sync.go` | Go / pgx | • PostgreSQL `LISTEN/NOTIFY`<br>• Monotonic Sequence (`tenants.revision`) gap detection<br>• Revision reconciliation and catch-up<br>• Cold start BadgerDB snapshot | • Request handling<br>• Telemetry parsing |
| **Audit Logger** | `internal/audit/` | Go / pgx | • Bounded asynchronous queueing<br>• Redaction and envelope encryption<br>• Encrypted PostgreSQL batch storage (`pgx.CopyFrom`)<br>• Encrypted spill/replay on failure | • Inline evaluation blocking<br>• Policy checking |

---

## 3. State Management & Distributed Resilience

### 3.1. Zero-Wait Stateless In-Memory Architecture
- **In-Memory State Representation**: Engine holds policies inside memory via atomic pointer swap:
  $$\text{EngineState} = \langle \text{RadixTrie}, \text{RoleDAG}, \text{RevocationMap} \rangle$$
- **Hot-Path Read Operation**: Zero locks (`sync.Mutex` absent on evaluation path). Evaluator executes via `atomic.LoadPointer` (Copy-On-Write COW semantics).

### 3.2. Monotonic Revision Sequence & Gap Detection
- **Database Sequence**: Atomic revision counter in PostgreSQL:
  ```sql
  UPDATE tenants SET revision = revision + 1 WHERE id = $1 RETURNING revision;
  ```
- **Event Protocol**: Payload sent via `NOTIFY policy_events, '{"tenant_id":"...", "revision": 42}'` (< 120 bytes).
- **Gap Detection Algorithm**:
  ```text
  On Event(tenant_id, event_rev):
    current_rev = LocalTenantRevision(tenant_id)
    if event_rev == current_rev + 1:
      ApplyDelta(event)
      LocalTenantRevision(tenant_id) = event_rev
    elif event_rev > current_rev + 1:
      // Gap detected (missed network packets)
      TriggerFastCatchUpSync(tenant_id, current_rev, event_rev)
    else:
      IgnoreStaleEvent(event)
  ```

### 3.3. Cold Start & Offline Resilience (Storage Profile)
- **Cloud Profile (`STORAGE_MODE=cloud`)**: 100% diskless pod startup; loads snapshot directly from PostgreSQL in $< 50$ms.
- **Edge / Air-Gapped Profile (`STORAGE_MODE=edge`)**: A versioned BadgerDB policy-source snapshot is recompiled and atomically installed before the listener opens, without PostgreSQL. An empty, legacy or invalid snapshot aborts startup. This constrained mode rejects delegated requests because durable revocation is unavailable, and it is not a production ERP deployment profile.
