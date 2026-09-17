package engine

import (
	"context"
	"testing"
	"time"

	"standalone-policy-engine/internal/storage"
)

func TestRestoreEdgeSnapshotsAfterOfflineRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &storage.PolicySnapshot{
		FormatVersion: storage.PolicySnapshotFormatVersion,
		TenantID:      "tenant-edge",
		Policies: []storage.PolicySnapshotPolicy{{
			ID:   "permit-reader",
			Text: `permit(principal == user:"alice", action == action:READ, resource == any);`,
		}},
		Inheritances: [][2]string{{"user:alice", "role:reader"}},
		Revision:     7,
		SnapshotAt:   time.Now(),
	}
	if err := store.SavePolicySnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	restartedStore, err := storage.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restartedStore.Close()
	eng := NewEngineWithGC(GCConfig{Enabled: false})
	if err := RestoreEdgeSnapshots(eng, restartedStore); err != nil {
		t.Fatal(err)
	}
	if got := eng.GetTenantRevision("tenant-edge"); got != snapshot.Revision {
		t.Fatalf("revision = %d, want %d", got, snapshot.Revision)
	}
	if got := eng.CheckPermission(context.Background(), "tenant-edge", "user:alice", "READ", "file:1", nil).Decision; got != DecisionAllow {
		t.Fatalf("restored decision = %s, want ALLOW", got)
	}
}

func TestRestoreEdgeSnapshotsFailsClosedWithoutSnapshot(t *testing.T) {
	store, err := storage.NewBadgerStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	eng := NewEngineWithGC(GCConfig{Enabled: false})
	if err := RestoreEdgeSnapshots(eng, store); err == nil {
		t.Fatal("expected empty edge store to prevent startup")
	}
	if len(eng.GetState().Tenants) != 0 {
		t.Fatal("failed restore must not publish a partial state")
	}
}
