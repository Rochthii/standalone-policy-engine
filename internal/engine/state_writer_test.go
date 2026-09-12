package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestEngineCOWWritersDoNotLoseDistinctTenants(t *testing.T) {
	const tenantCount = 128
	for round := 0; round < 10; round++ {
		eng := NewEngine()
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < tenantCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				tenantID := fmt.Sprintf("tenant-%03d", index)
				if err := eng.UpdateTenantPoliciesWithRevision(tenantID, nil, nil, 1); err != nil {
					t.Errorf("update %s: %v", tenantID, err)
				}
			}(i)
		}
		close(start)
		wg.Wait()

		if got := len(eng.GetState().Tenants); got != tenantCount {
			t.Fatalf("round %d: expected %d tenants, got %d", round, tenantCount, got)
		}
	}
}

func TestExplicitRevisionCannotMoveBackward(t *testing.T) {
	for _, test := range []struct {
		name   string
		update func(string, uint64) error
		state  func() *EngineState
	}{
		{
			name: "Engine",
			update: func(tenantID string, revision uint64) error {
				return coreRevisionTestEngine.UpdateTenantPoliciesWithRevision(tenantID, nil, nil, revision)
			},
			state: func() *EngineState { return coreRevisionTestEngine.GetState() },
		},
		{
			name: "EngineWithGC",
			update: func(tenantID string, revision uint64) error {
				return gcRevisionTestEngine.UpdateTenantPoliciesWithRevision(tenantID, nil, nil, revision)
			},
			state: func() *EngineState { return gcRevisionTestEngine.GetState() },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.update("tenant-revision", 10); err != nil {
				t.Fatalf("install revision 10: %v", err)
			}
			originalTrie := test.state().Tenants["tenant-revision"]
			if err := test.update("tenant-revision", 9); !errors.Is(err, ErrStaleRevision) {
				t.Fatalf("expected ErrStaleRevision, got %v", err)
			}
			if got := test.state().Tenants["tenant-revision"]; got != originalTrie || got.Revision != 10 {
				t.Fatalf("stale update changed state: trie_changed=%v revision=%d", got != originalTrie, got.Revision)
			}
			if err := test.update("tenant-revision", 10); err != nil {
				t.Fatalf("duplicate revision should be an idempotent no-op: %v", err)
			}
			if got := test.state().Tenants["tenant-revision"]; got != originalTrie {
				t.Fatal("duplicate revision replaced immutable tenant state")
			}
		})
	}
}

var (
	coreRevisionTestEngine = NewEngine()
	gcRevisionTestEngine   = NewEngineWithGC(GCConfig{Enabled: false})
)

func TestShouldSyncRevision(t *testing.T) {
	for _, test := range []struct {
		current, received uint64
		want              bool
	}{
		{current: 10, received: 0, want: true},
		{current: 10, received: 9, want: false},
		{current: 10, received: 10, want: false},
		{current: 10, received: 11, want: true},
		{current: 10, received: 15, want: true},
	} {
		if got := shouldSyncRevision(test.current, test.received); got != test.want {
			t.Fatalf("shouldSyncRevision(%d,%d)=%v, want %v", test.current, test.received, got, test.want)
		}
	}
}

func TestEngineAutomaticRevisionIsLinearizable(t *testing.T) {
	const updates = 128
	eng := NewEngine()
	if err := eng.UpdateTenantPolicies("tenant-a", nil, nil); err != nil {
		t.Fatalf("initial update: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < updates; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := eng.UpdateTenantPolicies("tenant-a", nil, nil); err != nil {
				t.Errorf("concurrent update: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got, want := eng.GetTenantRevision("tenant-a"), uint64(updates+1); got != want {
		t.Fatalf("expected revision %d, got %d", want, got)
	}
}

func TestEngineWithGCCOWWritersDoNotLoseDistinctTenants(t *testing.T) {
	const tenantCount = 128
	eng := NewEngineWithGC(GCConfig{Enabled: false})
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < tenantCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			tenantID := fmt.Sprintf("tenant-gc-%03d", index)
			if err := eng.UpdateTenantPoliciesWithRevision(tenantID, nil, nil, 1); err != nil {
				t.Errorf("update %s: %v", tenantID, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if got := len(eng.GetState().Tenants); got != tenantCount {
		t.Fatalf("expected %d tenants, got %d", tenantCount, got)
	}
}

func TestEngineWithGCAutomaticRevisionIsLinearizable(t *testing.T) {
	const updates = 128
	eng := NewEngineWithGC(GCConfig{Enabled: false})
	if err := eng.UpdateTenantPolicies("tenant-a", nil, nil); err != nil {
		t.Fatalf("initial update: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < updates; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := eng.UpdateTenantPolicies("tenant-a", nil, nil); err != nil {
				t.Errorf("concurrent update: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got, want := eng.GetTenantRevision("tenant-a"), uint64(updates+1); got != want {
		t.Fatalf("expected revision %d, got %d", want, got)
	}
}
