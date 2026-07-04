package engine

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"standalone-policy-engine/internal/parser"
)

// tenantAccessRecord ghi nhận thời điểm truy cập cuối và số lượng chính sách cho mỗi Tenant.
type tenantAccessRecord struct {
	lastAccess   time.Time
	policyCount  int
}

// GCConfig chứa cấu hình cho cơ chế GC dọn dẹp RAM của Engine.
type GCConfig struct {
	Enabled      bool
	Interval     time.Duration
	IdleTimeout  time.Duration
	MaxCacheSize int
}

// EngineWithGC mở rộng Engine cơ bản với cơ chế Tenant Cache GC và Lazy Loading.
type EngineWithGC struct {
	// Nhúng Engine cơ bản (COW + atomic pointer)
	state unsafe.Pointer

	// accessRecords theo dõi thời điểm truy cập cuối của từng Tenant
	accessRecords sync.Map // map[tenantID string]*tenantAccessRecord

	// lazyLoader là hàm callback được gọi khi một Tenant bị unload và có request mới đến
	// Caller (Syncer) sẽ cung cấp hàm này để tải lại dữ liệu từ Postgres
	lazyLoader func(ctx context.Context, tenantID string) error

	// gcEnabled là cờ bật/tắt GC
	gcEnabled bool

	// gcInterval là chu kỳ quét dọn dẹp (mặc định 1 giờ)
	gcInterval time.Duration

	// maxIdleTime là thời gian tối đa Tenant được phép không hoạt động (mặc định 24 giờ)
	maxIdleTime time.Duration

	stopGC chan struct{}
}

// NewEngineWithGC khởi tạo Engine có cơ chế GC tự động dọn dẹp Tenant không hoạt động.
func NewEngineWithGC(cfg GCConfig) *EngineWithGC {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	idle := cfg.IdleTimeout
	if idle <= 0 {
		idle = 24 * time.Hour
	}
	e := &EngineWithGC{
		gcEnabled:   cfg.Enabled,
		gcInterval:  interval,
		maxIdleTime: idle,
		stopGC:      make(chan struct{}),
	}
	initialState := &EngineState{
		Tenants: make(map[string]*TrieRoot),
	}
	atomic.StorePointer(&e.state, unsafe.Pointer(initialState))
	return e
}

// SetLazyLoader đăng ký hàm callback tải lại Tenant từ database khi bị unload.
func (e *EngineWithGC) SetLazyLoader(loader func(ctx context.Context, tenantID string) error) {
	e.lazyLoader = loader
}

// StartGC khởi chạy goroutine GC dọn dẹp Tenant không hoạt động.
func (e *EngineWithGC) StartGC(ctx context.Context) {
	if e.gcEnabled {
		go e.gcWorker(ctx)
	}
}

// StopGC dừng goroutine GC.
func (e *EngineWithGC) StopGC() {
	close(e.stopGC)
}

// GetState trả về EngineState hiện tại lock-free.
func (e *EngineWithGC) GetState() *EngineState {
	return (*EngineState)(atomic.LoadPointer(&e.state))
}

// GetTenantTrie lấy Trie của Tenant và cập nhật thời gian truy cập.
// Nếu Tenant bị unload, kích hoạt Lazy Loading và trả về sau khi load xong.
func (e *EngineWithGC) GetTenantTrie(ctx context.Context, tenantID string) (*TrieRoot, bool) {
	state := e.GetState()
	trie, exists := state.Tenants[tenantID]

	if exists {
		// Cập nhật thời gian truy cập
		e.touchTenant(tenantID, len(state.Tenants))
		return trie, true
	}

	// Tenant đã bị unload → kích hoạt Lazy Loading
	if e.lazyLoader != nil {
		log.Printf("[GC-Engine] Tenant %s đã bị unload khỏi RAM. Đang tải lại từ Postgres...", tenantID)
		if err := e.lazyLoader(ctx, tenantID); err != nil {
			log.Printf("[GC-Engine] Lỗi Lazy Loading Tenant %s: %v", tenantID, err)
			return nil, false
		}

		// Đọc lại state sau khi lazy load xong
		state = e.GetState()
		trie, exists = state.Tenants[tenantID]
		if exists {
			e.touchTenant(tenantID, 0)
		}
		return trie, exists
	}

	return nil, false
}

// CheckPermission là hot-path lock-free gọi trực tiếp qua Engine gốc (cho PDP gRPC server).
// Phương thức này cập nhật LastAccessTime cho GC tracking.
func (e *EngineWithGC) CheckPermission(ctx context.Context, tenantID, subject, action, resource string, ctxMap map[string]string) DecisionResult {
	if err := ctx.Err(); err != nil {
		return DecisionResult{
			Decision: DecisionDeny,
			Reason:   "Yêu cầu bị hủy hoặc hết thời gian chờ: " + err.Error(),
		}
	}

	state := e.GetState()
	trie, exists := state.Tenants[tenantID]
	if !exists {
		// Thử lấy và nạp từ Lazy Loading nếu có cấu hình
		var loaded bool
		trie, loaded = e.GetTenantTrie(ctx, tenantID)
		if !loaded {
			return DecisionResult{
				Decision: DecisionDeny,
				Reason:   "Tenant không tồn tại hoặc chưa được nạp chính sách",
			}
		}
	} else {
		// Cập nhật LastAccess
		e.touchTenant(tenantID, 0)
	}

	// Delegate sang decision engine core
	return CheckPermission(ctx, trie, subject, action, resource, ctxMap)
}

// UpdateTenantPolicies cập nhật tập luật cho Tenant sử dụng COW.
func (e *EngineWithGC) UpdateTenantPolicies(tenantID string, policies []*parser.PolicyNode, inheritances [][2]string) error {
	oldState := e.GetState()

	newState := &EngineState{
		Tenants: make(map[string]*TrieRoot, len(oldState.Tenants)+1),
	}
	for tid, trie := range oldState.Tenants {
		if tid != tenantID {
			newState.Tenants[tid] = trie
		}
	}

	newTrie := NewTrieRoot(tenantID)
	for _, pair := range inheritances {
		if err := newTrie.RoleDAG.AddInheritance(pair[0], pair[1]); err != nil {
			return err
		}
	}
	for _, policy := range policies {
		newTrie.AddPolicy(policy)
	}
	newState.Tenants[tenantID] = newTrie

	atomic.StorePointer(&e.state, unsafe.Pointer(newState))

	// Cập nhật record khi Tenant có chính sách mới
	e.touchTenant(tenantID, len(policies))
	return nil
}

// UnloadTenant xóa Trie của Tenant ra khỏi RAM (dùng bởi GC).
func (e *EngineWithGC) UnloadTenant(tenantID string) {
	oldState := e.GetState()

	if _, exists := oldState.Tenants[tenantID]; !exists {
		return
	}

	newState := &EngineState{
		Tenants: make(map[string]*TrieRoot, len(oldState.Tenants)-1),
	}
	for tid, trie := range oldState.Tenants {
		if tid != tenantID {
			newState.Tenants[tid] = trie
		}
	}

	atomic.StorePointer(&e.state, unsafe.Pointer(newState))
	e.accessRecords.Delete(tenantID)
	log.Printf("[GC-Engine] Đã unload Tenant %s ra khỏi RAM để giải phóng bộ nhớ.", tenantID)
}

// touchTenant cập nhật thời gian truy cập cuối của Tenant.
func (e *EngineWithGC) touchTenant(tenantID string, policyCount int) {
	record, _ := e.accessRecords.LoadOrStore(tenantID, &tenantAccessRecord{})
	r := record.(*tenantAccessRecord)
	r.lastAccess = time.Now()
	if policyCount > 0 {
		r.policyCount = policyCount
	}
}

// gcWorker là goroutine định kỳ dọn dẹp Tenant không hoạt động.
func (e *EngineWithGC) gcWorker(ctx context.Context) {
	ticker := time.NewTicker(e.gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopGC:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.runGCCycle()
		}
	}
}

func (e *EngineWithGC) runGCCycle() {
	now := time.Now()
	toUnload := make([]string, 0)

	e.accessRecords.Range(func(key, value interface{}) bool {
		tenantID := key.(string)
		record := value.(*tenantAccessRecord)
		if now.Sub(record.lastAccess) > e.maxIdleTime {
			toUnload = append(toUnload, tenantID)
		}
		return true
	})

	for _, tenantID := range toUnload {
		log.Printf("[GC-Engine] Tenant %s không hoạt động > %v. Đang unload khỏi RAM...", tenantID, e.maxIdleTime)
		e.UnloadTenant(tenantID)
	}

	if len(toUnload) > 0 {
		log.Printf("[GC-Engine] GC Cycle hoàn tất. Đã giải phóng %d Tenant khỏi RAM.", len(toUnload))
	}
}
