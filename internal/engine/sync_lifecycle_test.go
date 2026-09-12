package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"standalone-policy-engine/internal/storage"
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
