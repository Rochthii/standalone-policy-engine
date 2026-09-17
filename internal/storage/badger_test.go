package storage

import (
	"os"
	"testing"
	"time"
)

func TestBadgerStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("Không thể khởi tạo BadgerStore: %v", err)
	}
	defer store.Close()

	// Lưu snapshot
	snapshot := &PolicySnapshot{
		FormatVersion: PolicySnapshotFormatVersion,
		TenantID:      "tenant-test-001",
		Policies: []PolicySnapshotPolicy{
			{ID: "P-001", Text: `permit(principal == any, action == action:READ, resource == any);`},
			{ID: "P-002", Text: `forbid(principal == any, action == action:DELETE, resource == any);`},
		},
		Inheritances: [][2]string{
			{"user:alice", "role:admin"},
			{"role:admin", "role:operator"},
		},
		Revision:   1,
		SnapshotAt: time.Now(),
	}

	err = store.SavePolicySnapshot(snapshot)
	if err != nil {
		t.Fatalf("Lỗi lưu snapshot: %v", err)
	}

	// Tải lại snapshot
	loaded, err := store.LoadPolicySnapshot("tenant-test-001")
	if err != nil {
		t.Fatalf("Lỗi tải snapshot: %v", err)
	}
	if loaded == nil {
		t.Fatal("Snapshot bị nil sau khi tải lại từ BadgerDB")
	}

	if loaded.TenantID != snapshot.TenantID {
		t.Errorf("TenantID không khớp: mong đợi %s, thực tế %s", snapshot.TenantID, loaded.TenantID)
	}
	if len(loaded.Policies) != len(snapshot.Policies) {
		t.Errorf("Số lượng Policy không khớp: mong đợi %d, thực tế %d", len(snapshot.Policies), len(loaded.Policies))
	}
	if len(loaded.Inheritances) != len(snapshot.Inheritances) {
		t.Errorf("Số lượng Inheritance không khớp: mong đợi %d, thực tế %d", len(snapshot.Inheritances), len(loaded.Inheritances))
	}
}

func TestBadgerStoreKeyNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("Không thể khởi tạo BadgerStore: %v", err)
	}
	defer store.Close()

	// Tải một tenant không tồn tại → phải trả về nil, không lỗi
	loaded, err := store.LoadPolicySnapshot("tenant-not-exist")
	if err != nil {
		t.Fatalf("LoadPolicySnapshot không nên trả về lỗi khi key không tồn tại, nhưng trả về: %v", err)
	}
	if loaded != nil {
		t.Errorf("LoadPolicySnapshot phải trả về nil cho key không tồn tại, thực tế: %v", loaded)
	}
}

func TestBadgerStoreListTenantIDs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("Không thể khởi tạo BadgerStore: %v", err)
	}
	defer store.Close()

	tenants := []string{"tenant-A", "tenant-B", "tenant-C"}
	for _, id := range tenants {
		snap := &PolicySnapshot{FormatVersion: PolicySnapshotFormatVersion, TenantID: id, Revision: 1, SnapshotAt: time.Now()}
		if err := store.SavePolicySnapshot(snap); err != nil {
			t.Fatalf("Lỗi lưu snapshot cho %s: %v", id, err)
		}
	}

	ids, err := store.ListTenantIDs()
	if err != nil {
		t.Fatalf("Lỗi ListTenantIDs: %v", err)
	}
	if len(ids) != len(tenants) {
		t.Errorf("Mong đợi %d tenants, thực tế %d", len(tenants), len(ids))
	}
}

func TestBadgerStoreDeleteSnapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("Không thể khởi tạo BadgerStore: %v", err)
	}
	defer store.Close()

	snap := &PolicySnapshot{FormatVersion: PolicySnapshotFormatVersion, TenantID: "tenant-del", Revision: 1, SnapshotAt: time.Now()}
	_ = store.SavePolicySnapshot(snap)

	// Xóa snapshot
	err = store.DeleteTenantSnapshot("tenant-del")
	if err != nil {
		t.Fatalf("Lỗi xóa snapshot: %v", err)
	}

	// Xác nhận đã bị xóa
	loaded, err := store.LoadPolicySnapshot("tenant-del")
	if err != nil {
		t.Fatalf("Lỗi sau khi xóa: %v", err)
	}
	if loaded != nil {
		t.Error("Snapshot phải là nil sau khi xóa")
	}
}

func TestBadgerStoreRejectsUnrestorableSnapshot(t *testing.T) {
	store, err := NewBadgerStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SavePolicySnapshot(&PolicySnapshot{TenantID: "tenant-a"}); err == nil {
		t.Fatal("expected incomplete snapshot to be rejected")
	}
}

func TestBadgerStorePathCreation(t *testing.T) {
	dir := t.TempDir() + "/nested/dir"
	store, err := NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("Phải tự tạo thư mục nested, nhưng lỗi: %v", err)
	}
	defer store.Close()

	if _, statErr := os.Stat(store.StorePath()); os.IsNotExist(statErr) {
		t.Error("Thư mục BadgerDB phải được tạo tự động")
	}
}
