package engine

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/storage"
)

// RestoreEdgeSnapshots recompiles every durable Badger policy snapshot and
// atomically installs the resulting tenant state before the data plane serves.
func RestoreEdgeSnapshots(eng *EngineWithGC, snapshots *storage.BadgerStore) error {
	if eng == nil || snapshots == nil {
		return fmt.Errorf("edge restore requires engine and Badger store")
	}
	if len(eng.GetState().Tenants) != 0 {
		return fmt.Errorf("edge restore requires an empty engine")
	}
	tenantIDs, err := snapshots.ListTenantIDs()
	if err != nil {
		return fmt.Errorf("list edge policy snapshots: %w", err)
	}
	if len(tenantIDs) == 0 {
		return fmt.Errorf("no edge policy snapshots available")
	}

	restored := make(map[string]*TrieRoot, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		snapshot, err := snapshots.LoadPolicySnapshot(tenantID)
		if err != nil {
			return fmt.Errorf("load edge policy snapshot for tenant %s: %w", tenantID, err)
		}
		if snapshot == nil || snapshot.TenantID != tenantID {
			return fmt.Errorf("edge policy snapshot is missing or mismatched for tenant %s", tenantID)
		}
		sources := make([]parser.PolicySource, 0, len(snapshot.Policies))
		for _, policy := range snapshot.Policies {
			sources = append(sources, parser.PolicySource{ID: policy.ID, Text: policy.Text})
		}
		policies, err := parser.CompilePolicySet(sources)
		if err != nil {
			return fmt.Errorf("compile edge policy snapshot for tenant %s: %w", tenantID, err)
		}
		trie, err := buildTenantTrie(tenantID, policies, snapshot.Inheritances, snapshot.Revision)
		if err != nil {
			return fmt.Errorf("build edge policy state for tenant %s: %w", tenantID, err)
		}
		restored[tenantID] = trie
	}

	eng.writeMu.Lock()
	defer eng.writeMu.Unlock()
	if len(eng.GetState().Tenants) != 0 {
		return fmt.Errorf("edge restore requires an empty engine")
	}
	atomic.StorePointer(&eng.state, unsafe.Pointer(&EngineState{Tenants: restored}))
	for tenantID, trie := range restored {
		eng.touchTenant(tenantID, len(trie.GlobalPolicies))
	}
	return nil
}
