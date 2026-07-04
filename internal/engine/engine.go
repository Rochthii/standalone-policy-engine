package engine

import (
	"sync/atomic"
	"unsafe"
	"standalone-policy-engine/internal/parser"
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

// UpdateTenantPolicies cập nhật tập luật và phân cấp vai trò cho một Tenant cụ thể.
// Áp dụng cơ chế Copy-On-Write (COW):
//  1. Nhân bản map Tenants cũ sang map mới.
//  2. Xây dựng lại toàn bộ TrieRoot mới cho Tenant cần cập nhật (nạp vai trò và chính sách).
//  3. Hoán đổi con trỏ nguyên tử (Atomic Pointer Swap) sang state mới.
func (e *Engine) UpdateTenantPolicies(tenantID string, policies []*parser.PolicyNode, inheritances [][2]string) error {
	oldState := e.GetState()

	// 1. Tạo EngineState mới
	newState := &EngineState{
		Tenants: make(map[string]*TrieRoot),
	}

	// 2. Sao chép nông (shallow copy) các con trỏ Trie của các Tenant khác
	for tid, trie := range oldState.Tenants {
		if tid != tenantID {
			newState.Tenants[tid] = trie
		}
	}

	// 3. Xây dựng mới hoàn toàn TrieRoot cho Tenant được cập nhật
	newTrie := NewTrieRoot(tenantID)

	// Nạp phân cấp vai trò DAG trước
	for _, pair := range inheritances {
		parent := pair[0]
		child := pair[1]
		if err := newTrie.RoleDAG.AddInheritance(parent, child); err != nil {
			return err
		}
	}

	// Nạp các chính sách vào Trie
	for _, policy := range policies {
		newTrie.AddPolicy(policy)
	}

	// Đưa Trie mới vào state mới
	newState.Tenants[tenantID] = newTrie

	// 4. Hoán đổi con trỏ nguyên tử
	atomic.StorePointer(&e.state, unsafe.Pointer(newState))

	return nil
}
