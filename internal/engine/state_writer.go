package engine

import (
	"errors"

	"standalone-policy-engine/internal/parser"
)

var ErrStaleRevision = errors.New("stale policy revision")

func revisionIsStale(oldState *EngineState, tenantID string, revision uint64) bool {
	current, exists := oldState.Tenants[tenantID]
	return exists && revision > 0 && revision < current.Revision
}

func revisionIsDuplicate(oldState *EngineState, tenantID string, revision uint64) bool {
	current, exists := oldState.Tenants[tenantID]
	return exists && revision > 0 && revision == current.Revision
}

func buildTenantTrie(tenantID string, policies []*parser.PolicyNode, inheritances [][2]string, revision uint64) (*TrieRoot, error) {
	trie := NewTrieRoot(tenantID)
	trie.Revision = revision
	for _, pair := range inheritances {
		if err := trie.RoleDAG.AddInheritance(pair[0], pair[1]); err != nil {
			return nil, err
		}
	}
	for _, policy := range policies {
		trie.AddPolicy(policy)
	}
	return trie, nil
}

func stateWithTenant(oldState *EngineState, tenantID string, tenantTrie *TrieRoot) *EngineState {
	newState := &EngineState{Tenants: make(map[string]*TrieRoot, len(oldState.Tenants)+1)}
	for existingTenantID, trie := range oldState.Tenants {
		if existingTenantID != tenantID {
			newState.Tenants[existingTenantID] = trie
		}
	}
	newState.Tenants[tenantID] = tenantTrie
	return newState
}

func stateWithoutTenant(oldState *EngineState, tenantID string) *EngineState {
	newState := &EngineState{Tenants: make(map[string]*TrieRoot, len(oldState.Tenants)-1)}
	for existingTenantID, trie := range oldState.Tenants {
		if existingTenantID != tenantID {
			newState.Tenants[existingTenantID] = trie
		}
	}
	return newState
}
