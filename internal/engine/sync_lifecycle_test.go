package engine

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"standalone-policy-engine/internal/storage"
	"standalone-policy-engine/internal/testutil"
)

type blockingSyncStore struct {
	listenStarted chan struct{}
	reconciles    atomic.Int64
}

func (s *blockingSyncStore) ListenPolicyEvents(ctx context.Context, _ func(storage.DBPolicyUpdateEvent)) error {
	select {
	case <-s.listenStarted:
	default:
		close(s.listenStarted)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (s *blockingSyncStore) GetTenantRevision(_ context.Context, _ string) (uint64, error) {
	s.reconciles.Add(1)
	return 1, nil
}

func (s *blockingSyncStore) GetTenantPolicyBundle(_ context.Context, _ string) (*storage.TenantPolicyBundle, error) {
	return &storage.TenantPolicyBundle{Revision: 1}, nil
}

func TestSyncerStopCancelsBlockingListener(t *testing.T) {
	store := &blockingSyncStore{listenStarted: make(chan struct{})}
	syncer := NewSyncer(NewEngineWithGC(GCConfig{Enabled: false}), store, time.Hour)
	syncer.Start(context.Background())
	select {
	case <-store.listenStarted:
	case <-time.After(time.Second):
		t.Fatal("listener did not start")
	}

	stopped := make(chan struct{})
	go func() {
		syncer.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop blocked instead of canceling listener context")
	}
}

func TestSyncerReconcilesWhileListenerIsHealthy(t *testing.T) {
	store := &blockingSyncStore{listenStarted: make(chan struct{})}
	eng := NewEngineWithGC(GCConfig{Enabled: false})
	if err := eng.UpdateTenantPoliciesWithRevision("tenant-a", nil, nil, 1); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	syncer := NewSyncer(eng, store, 5*time.Millisecond)
	syncer.Start(context.Background())
	defer syncer.Stop()

	deadline := time.Now().Add(time.Second)
	for store.reconciles.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if store.reconciles.Load() == 0 {
		t.Fatal("periodic reconciliation did not run while LISTEN remained connected")
	}
}

func TestSyncerRestartsAndReconcilesRoleInheritance(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL role sync integration")
	}
	ctx := context.Background()
	store, err := storage.NewStorage(testutil.CreateIsolatedPostgresDatabase(t, ctx, adminURL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	tenantID, err := store.CreateTenant(ctx, "tenant-role-sync")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := store.ReplaceRoleInheritances(ctx, tenantID, [][2]string{{"user:alice", "role:manager"}, {"role:manager", "role:staff"}}); err != nil {
		t.Fatalf("persist initial role inheritance: %v", err)
	}
	initialRevision, err := store.GetTenantRevision(ctx, tenantID)
	if err != nil {
		t.Fatalf("read initial role revision: %v", err)
	}
	restartedEngine := NewEngineWithGC(GCConfig{Enabled: false})
	restartedSyncer := NewSyncer(restartedEngine, store, time.Hour)
	if err := restartedSyncer.SyncTenant(ctx, tenantID); err != nil {
		t.Fatalf("reload role bundle after restart: %v", err)
	}
	trie := restartedEngine.GetState().Tenants[tenantID]
	if trie == nil || trie.Revision != initialRevision || !trie.RoleDAG.IsDescendant("user:alice", "role:staff") {
		t.Fatalf("restart did not rebuild role DAG: %#v", trie)
	}

	if err := store.ReplaceRoleInheritances(ctx, tenantID, [][2]string{{"user:alice", "role:auditor"}}); err != nil {
		t.Fatalf("persist missed role event: %v", err)
	}
	catchUpRevision, err := store.GetTenantRevision(ctx, tenantID)
	if err != nil || catchUpRevision <= initialRevision {
		t.Fatalf("read catch-up role revision: initial=%d current=%d err=%v", initialRevision, catchUpRevision, err)
	}
	if err := restartedSyncer.reconcileTenantRevision(ctx, tenantID); err != nil {
		t.Fatalf("catch up missed role event: %v", err)
	}
	trie = restartedEngine.GetState().Tenants[tenantID]
	if trie == nil || trie.Revision != catchUpRevision || !trie.RoleDAG.IsDescendant("user:alice", "role:auditor") || trie.RoleDAG.IsDescendant("user:alice", "role:staff") {
		t.Fatalf("missed-event catch-up did not replace role DAG: %#v", trie)
	}
}
