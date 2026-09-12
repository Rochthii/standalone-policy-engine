package engine

import (
	"fmt"
	"standalone-policy-engine/internal/parser"
	"sync"
	"sync/atomic"
	"unsafe"
)

// EngineState chứa toàn bộ trạng thái của Data Plane (PDP) trên bộ nhớ RAM.
// Trạng thái này là bất biến (Immutable), mỗi lần cập nhật sẽ hoán đổi toàn bộ state.
type EngineState struct {
	// Tenants map chứa TrieRoot của từng Tenant.
	Tenants map[string]*TrieRoot
}

// Engine là động cơ PDP điều phối các hoạt động tra cứu, đánh giá và quyết định phân quyền.
type Engine struct {
	// state là con trỏ unsafe.Pointer trỏ tới struct *EngineState.
	// Cho phép luồng đọc CheckAccess hoàn toàn lock-free sử dụng atomic.LoadPointer.
	state unsafe.Pointer

	// writeMu serializes COW snapshots. Readers never acquire it.
	writeMu sync.Mutex
}

// NewEngine khởi tạo mới một PDP Engine trống.
func NewEngine() *Engine {
	e := &Engine{}
	initialState := &EngineState{
		Tenants: make(map[string]*TrieRoot),
	}
	atomic.StorePointer(&e.state, unsafe.Pointer(initialState))
	return e
}

// GetState truy xuất trạng thái EngineState hiện tại một cách an toàn đa luồng lock-free.
func (e *Engine) GetState() *EngineState {
	return (*EngineState)(atomic.LoadPointer(&e.state))
}

// GetTenantTrie lấy cây TrieRoot của một Tenant cụ thể từ state hiện tại.
func (e *Engine) GetTenantTrie(tenantID string) (*TrieRoot, bool) {
	state := e.GetState()
	trie, exists := state.Tenants[tenantID]
	return trie, exists
}

// GetTenantRevision lấy số hiệu phiên bản (Revision ID) hiện tại của một Tenant.
func (e *Engine) GetTenantRevision(tenantID string) uint64 {
	if trie, exists := e.GetTenantTrie(tenantID); exists {
		return trie.Revision
	}
	return 0
}

// GetTenantSchema trả về danh sách các thuộc tính biến mà tập chính sách của Tenant này thực sự cần.
func (e *Engine) GetTenantSchema(tenantID string) []string {
	if trie, exists := e.GetTenantTrie(tenantID); exists {
		return trie.RequiredAttributes
	}
	return nil
}

// UpdateTenantPolicies cập nhật tập luật và phân cấp vai trò cho một Tenant cụ thể (tự động tăng revision).
func (e *Engine) UpdateTenantPolicies(tenantID string, policies []*parser.PolicyNode, inheritances [][2]string) error {
	newTrie, err := buildTenantTrie(tenantID, policies, inheritances, 0)
	if err != nil {
		return err
	}

	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	oldState := e.GetState()
	if current, exists := oldState.Tenants[tenantID]; exists {
		newTrie.Revision = current.Revision + 1
	} else {
		newTrie.Revision = 1
	}
	atomic.StorePointer(&e.state, unsafe.Pointer(stateWithTenant(oldState, tenantID, newTrie)))
	return nil
}

// UpdateTenantPoliciesWithRevision cập nhật tập luật và phân cấp vai trò cho một Tenant với số hiệu Revision cụ thể.
// Áp dụng cơ chế Copy-On-Write (COW):
//  1. Nhân bản map Tenants cũ sang map mới.
//  2. Xây dựng lại toàn bộ TrieRoot mới cho Tenant cần cập nhật (nạp vai trò và chính sách).
//  3. Hoán đổi con trỏ nguyên tử (Atomic Pointer Swap) sang state mới.
func (e *Engine) UpdateTenantPoliciesWithRevision(tenantID string, policies []*parser.PolicyNode, inheritances [][2]string, revision uint64) error {
	newTrie, err := buildTenantTrie(tenantID, policies, inheritances, revision)
	if err != nil {
		return err
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	oldState := e.GetState()
	if revisionIsStale(oldState, tenantID, revision) {
		return fmt.Errorf("%w: tenant=%s current=%d received=%d", ErrStaleRevision, tenantID, oldState.Tenants[tenantID].Revision, revision)
	}
	if revisionIsDuplicate(oldState, tenantID, revision) {
		return nil
	}
	atomic.StorePointer(&e.state, unsafe.Pointer(stateWithTenant(oldState, tenantID, newTrie)))
	return nil
}
