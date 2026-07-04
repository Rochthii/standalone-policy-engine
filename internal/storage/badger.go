package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

// BadgerStore là tầng lưu trữ cục bộ nhúng (Embedded KV Database) dùng BadgerDB.
// Phục vụ kịch bản PDP Sidecar khởi động nhanh khi không kết nối được PostgreSQL.
type BadgerStore struct {
	db  *badger.DB
	dir string
}

// PolicySnapshot là cấu trúc dữ liệu lưu trữ bản sao JSON tập chính sách của một Tenant.
type PolicySnapshot struct {
	TenantID    string          `json:"tenant_id"`
	Policies    []json.RawMessage `json:"policies"`
	Inheritances [][2]string    `json:"inheritances"`
	SnapshotAt  time.Time       `json:"snapshot_at"`
}

// NewBadgerStore khởi tạo database BadgerDB cục bộ tại đường dẫn chỉ định.
func NewBadgerStore(dir string) (*BadgerStore, error) {
	// Đảm bảo thư mục tồn tại
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("không thể tạo thư mục BadgerDB: %w", err)
	}

	opts := badger.DefaultOptions(dir)
	// Tắt logging mặc định để tránh nhiễu output
	opts.Logger = nil
	// Tối ưu hóa cho workload read-heavy, ít write
	opts.ValueLogMaxEntries = 10000

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("không thể mở BadgerDB: %w", err)
	}

	log.Printf("[BadgerStore] Khởi tạo Edge Storage cục bộ thành công tại %s", dir)
	return &BadgerStore{db: db, dir: dir}, nil
}

// Close đóng kết nối BadgerDB an toàn.
func (b *BadgerStore) Close() error {
	return b.db.Close()
}

// SavePolicySnapshot lưu bản sao tập chính sách JSON của một Tenant xuống BadgerDB cục bộ.
// Được gọi sau mỗi lần đồng bộ hoàn thành thành công từ PostgreSQL.
func (b *BadgerStore) SavePolicySnapshot(snapshot *PolicySnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("lỗi serialize snapshot: %w", err)
	}

	key := policySnapshotKey(snapshot.TenantID)

	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

// LoadPolicySnapshot tải bản sao tập chính sách của một Tenant từ BadgerDB cục bộ.
// Trả về nil nếu không tìm thấy snapshot.
func (b *BadgerStore) LoadPolicySnapshot(tenantID string) (*PolicySnapshot, error) {
	key := policySnapshotKey(tenantID)

	var snapshot PolicySnapshot
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err // badger.ErrKeyNotFound sẽ được xử lý ở caller
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &snapshot)
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lỗi đọc snapshot từ BadgerDB: %w", err)
	}

	return &snapshot, nil
}

// ListTenantIDs liệt kê tất cả các TenantID hiện có snapshot trong BadgerDB.
func (b *BadgerStore) ListTenantIDs() ([]string, error) {
	prefix := []byte("policy:tenant:")
	tenantIDs := make([]string, 0)

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Key-only iteration
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			// Trích xuất tenantID từ key format "policy:tenant:{id}"
			tenantID := string(key[len(prefix):])
			tenantIDs = append(tenantIDs, tenantID)
		}
		return nil
	})

	return tenantIDs, err
}

// DeleteTenantSnapshot xóa snapshot của một Tenant khi không còn cần thiết.
func (b *BadgerStore) DeleteTenantSnapshot(tenantID string) error {
	key := policySnapshotKey(tenantID)
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// RunGC chạy garbage collection định kỳ để thu hồi dung lượng đĩa.
func (b *BadgerStore) RunGC() {
	// BadgerDB GC cần được gọi định kỳ để xóa các value log cũ
	for {
		err := b.db.RunValueLogGC(0.5)
		if err == badger.ErrNoRewrite {
			break
		}
		if err != nil {
			break
		}
	}
}

// policySnapshotKey tạo key BadgerDB cho snapshot chính sách của Tenant.
func policySnapshotKey(tenantID string) []byte {
	return []byte(fmt.Sprintf("policy:tenant:%s", tenantID))
}

// StorePath trả về đường dẫn thư mục của BadgerDB.
func (b *BadgerStore) StorePath() string {
	return filepath.Clean(b.dir)
}
