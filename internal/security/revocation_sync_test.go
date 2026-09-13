package security

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type snapshotRevocationStore struct {
	records []RevocationRecord
	err     error
}

type flappingRevocationStore struct {
	calls      atomic.Int32
	disconnect chan struct{}
}

func (*flappingRevocationStore) PersistRevocation(context.Context, RevocationRecord) (RevocationRecord, error) {
	return RevocationRecord{}, errors.New("not used")
}

func (s *flappingRevocationStore) WatchRevocations(ctx context.Context, snapshot func([]RevocationRecord), _ func(RevocationRecord)) error {
	if s.calls.Add(1) == 1 {
		snapshot(nil)
		select {
		case <-s.disconnect:
			return errors.New("listener disconnected")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

func (*snapshotRevocationStore) PersistRevocation(context.Context, RevocationRecord) (RevocationRecord, error) {
	return RevocationRecord{}, errors.New("not used")
}

func (s *snapshotRevocationStore) WatchRevocations(ctx context.Context, snapshot func([]RevocationRecord), _ func(RevocationRecord)) error {
	if s.err != nil {
		return s.err
	}
	snapshot(s.records)
	<-ctx.Done()
	return ctx.Err()
}

func TestRevocationSyncerLoadsSnapshotBeforeReady(t *testing.T) {
	now := time.Now()
	manager := NewDelegationManagerWithSecret("test-secret")
	store := &snapshotRevocationStore{records: []RevocationRecord{{
		TenantID: "tenant-a", GrantID: "grant-a", RevokedBy: "user:manager",
		RevokedAt: now, ExpiresAt: now.Add(time.Hour),
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	syncer := NewRevocationSyncer(manager, store)
	if err := syncer.Start(ctx); err != nil {
		t.Fatalf("start syncer: %v", err)
	}
	if !manager.IsRevoked("tenant-a", "grant-a") {
		t.Fatal("Start returned before the durable snapshot was installed")
	}
	cancel()
	syncer.Wait()
}

func TestRevocationSyncerFailsStartupWithoutSnapshot(t *testing.T) {
	manager := NewDelegationManagerWithSecret("test-secret")
	syncer := NewRevocationSyncer(manager, &snapshotRevocationStore{err: errors.New("database unavailable")})
	if err := syncer.Start(context.Background()); err == nil {
		t.Fatal("startup must fail closed when the durable snapshot cannot load")
	}
}

func TestRevocationSyncerMarksStateUnavailableOnDisconnect(t *testing.T) {
	manager := NewDelegationManagerWithSecret("test-secret")
	store := &flappingRevocationStore{disconnect: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	syncer := NewRevocationSyncer(manager, store)
	if err := syncer.Start(ctx); err != nil {
		t.Fatalf("start syncer: %v", err)
	}
	close(store.disconnect)
	deadline := time.Now().Add(time.Second)
	for manager.RevocationReady() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.RevocationReady() {
		t.Fatal("revocation state remained ready after listener disconnect")
	}
	cancel()
	syncer.Wait()
}

func TestApplyRevocationCannotBeShortenedByLateEvent(t *testing.T) {
	now := time.Now()
	manager := NewDelegationManagerWithSecret("test-secret")
	manager.ApplyRevocation(RevocationRecord{
		TenantID: "tenant-a", GrantID: "grant-a", RevokedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	manager.ApplyRevocation(RevocationRecord{
		TenantID: "tenant-a", GrantID: "grant-a", RevokedAt: now.Add(time.Minute), ExpiresAt: now.Add(-time.Second),
	})
	if !manager.IsRevoked("tenant-a", "grant-a") {
		t.Fatal("late or expired event shortened an active revocation")
	}
}
