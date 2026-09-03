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
│ 3. Cryptographic Verification: HMAC-SHA256(delegation_proof) (~1.2 µs)      │
│ 4. TOCTOU Mitigation: In-Memory RevocationMap O(1) Blacklist Lookup (<1 µs)│
│    └─ If revoked -> Short-circuit Hard DENY                                 │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │ Verified Clean Request Context
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ PDP CORE: In-Memory Decision Engine (Tầng 2 - Hot-Path Core: 27ns)          │
├─────────────────────────────────────────────────────────────────────────────┤
│ 1. Radix Trie Policy Filtering (FNV-1a 64-bit uint64 index):               │
│    TenantID -> SubjectHash -> ResourceHash -> ActionHash                    │
│ 2. Role DAG Transitive Closure Query: O(1) IsDescendant()                   │
│ 3. AST Pure Evaluation: sync.Pool EvalContext, Stack Scratch [64],          │
│    Zero Heap Allocation (0 allocs/op, 0 B/op)                               │
│ 4. Decision Synthesis: Deny-by-Default + Forbid-Overrides                   │
│ 5. Obligation Mapping: Matched Policy -> Obligations/Advice                 │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │ Decision Tuple: <Decision, Obligations>
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ AUDIT PIPELINE: Async Non-Blocking Telemetry                               │
├─────────────────────────────────────────────────────────────────────────────┤
│ Lock-Free Ring Buffer (1M slots) -> UDS Datagram Socket -> Vector Sidecar   │
│ -> PostgreSQL WORM Storage (pgx.CopyFrom Batch Insert)                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼ gRPC Response (< 0.35ms E2E)
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
| **gRPC Security Interceptor** | `internal/security/auth.go` | Go 1.22 / gRPC Server | • mTLS X.509 handshake<br>• JWT claims validation<br>• HMAC-SHA256 proof verification<br>• In-Memory `RevocationMap` $O(1)$ query | • AST policy evaluation<br>• Radix Trie indexing<br>• Disk I/O |
| **In-Memory Decision Engine** | `internal/engine/` | Go 1.22 (Zero-Alloc Hot-Path) | • Multi-level Radix Trie index<br>• Precomputed Role DAG closure $O(1)$<br>• Pure AST evaluation (27ns)<br>• Decision synthesis & Obligation attachment | • Cryptographic hashing<br>• Database queries<br>• Network protocol handling |
| **State Synchronizer** | `internal/storage/`, `internal/engine/sync.go` | Go 1.22 / pgx | • PostgreSQL `LISTEN/NOTIFY`<br>• Monotonic Sequence (`tenants.revision`) gap detection<br>• Replay Ring Buffer catch-up (< 50ms)<br>• Cold start BadgerDB snapshot | • Request handling<br>• Telemetry parsing |
| **Audit Logger** | `internal/audit/` | Go 1.22 / Vector / pgx | • Lock-free Ring Buffer queueing<br>• UDS Datagram dispatch<br>• Batch WORM storage (`pgx.CopyFrom`)<br>• Spill-to-Disk on failure (1GB cap) | • Inline evaluation blocking<br>• Policy checking |

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
- **Edge / Air-Gapped Profile (`STORAGE_MODE=edge`)**: Embedded BadgerDB KV store snapshot cached on local NVMe, boots instantly without network dependency.
