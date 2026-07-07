package engine

import (
	"standalone-policy-engine/internal/parser"
	"sync"
)

// policySlicePool tái sử dụng slice kết quả LookupPolicies để triệt tiêu heap allocation
// trên hot path. Caller phải gọi ReturnPolicySlice() sau khi xử lý xong.
var policySlicePool = sync.Pool{
	New: func() any {
		s := make([]*parser.PolicyNode, 0, 8)
		return &s
	},
}

// GetPolicySlice lấy một slice buffer từ pool. Caller PHẢI gọi ReturnPolicySlice sau khi dùng.
func GetPolicySlice() *[]*parser.PolicyNode {
	return policySlicePool.Get().(*[]*parser.PolicyNode)
}

// ReturnPolicySlice trả buffer về pool và reset độ dài về 0 (giữ nguyên capacity).
func ReturnPolicySlice(s *[]*parser.PolicyNode) {
	*s = (*s)[:0]
	policySlicePool.Put(s)
}

// TrieRoot là nút gốc chỉ mục RAM cho một Tenant cụ thể.
type TrieRoot struct {
	TenantID string

	// Subjects map mã băm FNV-1a của principal (ví dụ: hash("user:alice"))
	// trỏ tới danh sách các tài nguyên tương ứng.
	Subjects map[uint64]*SubjectNode

	// GlobalPolicies chứa các chính sách toàn cục (Global Rules Partition)
	// nơi cả principal và resource đều được thiết lập là 'any' (wildcard kép).
	GlobalPolicies []*parser.PolicyNode

	// RoleDAG quản lý phân cấp vai trò cho riêng Tenant này.
	RoleDAG *RoleDAG
}

// SubjectNode đại diện cho phân cấp Subject trong Trie.
type SubjectNode struct {
	// Resources map mã băm FNV-1a của resource (ví dụ: hash("file:report.pdf"))
	Resources map[uint64]*ResourceNode
}

// ResourceNode đại diện cho phân cấp Resource trong Trie.
type ResourceNode struct {
	// Actions map mã băm FNV-1a của action (ví dụ: hash("action:READ"))
	Actions map[uint64]*ActionNode
}

// ActionNode đại diện cho phân cấp Action trong Trie (nút lá chứa chính sách).
type ActionNode struct {
	Policies []*parser.PolicyNode
}

// NewTrieRoot tạo mới một instance TrieRoot cho một Tenant.
func NewTrieRoot(tenantID string) *TrieRoot {
	return &TrieRoot{
		TenantID:       tenantID,
		Subjects:       make(map[uint64]*SubjectNode),
		GlobalPolicies: make([]*parser.PolicyNode, 0),
		RoleDAG:        NewRoleDAG(),
	}
}

// fnvHash tính toán mã băm FNV-1a 64-bit cho chuỗi, tối ưu hóa zero-allocation.
func fnvHash(s string) uint64 {
	var hash uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= 1099511628211
	}
	return hash
}

// buildKeyHash chuyển đổi ScopeNode tĩnh thành mã băm uint64 trực tiếp.
func buildKeyHash(scope *parser.ScopeNode) uint64 {
	if scope == nil || scope.Operator == parser.ScopeOpAny {
		return fnvHash("any")
	}
	// Tránh dùng fmt.Sprintf bằng cách tự nối chuỗi đơn giản
	return fnvHash(scope.EntityType + ":" + scope.EntityID)
}

// AddPolicy nạp một chính sách đã compiled vào cây Trie.
func (t *TrieRoot) AddPolicy(policy *parser.PolicyNode) {
	if policy == nil {
		return
	}

	// 1. Kiểm tra cơ chế phân tách luật toàn cục (Global Rules Partition)
	// Nếu cả principal và resource đều là wildcard 'any', lưu vào phân vùng đặc biệt.
	isPrincipalAny := policy.Principal == nil || policy.Principal.Operator == parser.ScopeOpAny
	isResourceAny := policy.Resource == nil || policy.Resource.Operator == parser.ScopeOpAny

	if isPrincipalAny && isResourceAny {
		t.GlobalPolicies = append(t.GlobalPolicies, policy)
		return
	}

	// 2. Đi theo luồng phân cấp: Subject -> Resource -> Action (dùng uint64 hash)
	subHash := buildKeyHash(policy.Principal)
	resHash := buildKeyHash(policy.Resource)
	actHash := buildKeyHash(policy.Action)

	// Lấy hoặc tạo SubjectNode
	subNode, exists := t.Subjects[subHash]
	if !exists {
		subNode = &SubjectNode{Resources: make(map[uint64]*ResourceNode)}
		t.Subjects[subHash] = subNode
	}

	// Lấy hoặc tạo ResourceNode
	resNode, exists := subNode.Resources[resHash]
	if !exists {
		resNode = &ResourceNode{Actions: make(map[uint64]*ActionNode)}
		subNode.Resources[resHash] = resNode
	}

	// Lấy hoặc tạo ActionNode
	actNode, exists := resNode.Actions[actHash]
	if !exists {
		actNode = &ActionNode{Policies: make([]*parser.PolicyNode, 0)}
		resNode.Actions[actHash] = actNode
	}

	// Thêm chính sách vào danh sách nút lá
	actNode.Policies = append(actNode.Policies, policy)
}

// LookupPolicies tra cứu và thu thập tất cả các chính sách khớp với ngữ cảnh tĩnh của yêu cầu.
// Đây là API tương thích ngược, phân bổ slice mới mỗi lần.
// Với hiệu năng cao hơn, dùng LookupPoliciesInto kết hợp GetPolicySlice/ReturnPolicySlice.
func (t *TrieRoot) LookupPolicies(subjects []string, resources []string, action string) []*parser.PolicyNode {
	buf := GetPolicySlice()
	t.LookupPoliciesInto(buf, subjects, resources, action)
	result := make([]*parser.PolicyNode, len(*buf))
	copy(result, *buf)
	ReturnPolicySlice(buf)
	return result
}

// LookupPoliciesInto là phiên bản zero-allocation của LookupPolicies.
// Kết quả được append vào buf thay vì tạo slice mới.
// Sử dụng so sánh các mã băm uint64 để có hiệu năng CPU vượt trội.
func (t *TrieRoot) LookupPoliciesInto(buf *[]*parser.PolicyNode, subjects []string, resources []string, action string) {
	// 1. Luôn gộp các chính sách toàn cục (Global Rules Partition)
	*buf = append(*buf, t.GlobalPolicies...)

	// Đảm bảo "any" luôn nằm trong danh sách để so khớp wildcard.
	var subScratch [8]string
	var resScratch [8]string
	subjectsWithAny := ensureAnyInto(subjects, &subScratch)
	resourcesWithAny := ensureAnyInto(resources, &resScratch)

	// Cache mã băm của action và "any" trên stack
	actionHash := fnvHash(action)
	anyHash := fnvHash("any")

	// 2. Tra cứu trên cây Trie theo tất cả các tổ hợp danh tính hợp lệ
	for _, subKey := range subjectsWithAny {
		subHash := fnvHash(subKey)
		subNode, exists := t.Subjects[subHash]
		if !exists {
			continue
		}

		for _, resKey := range resourcesWithAny {
			resHash := fnvHash(resKey)
			resNode, exists := subNode.Resources[resHash]
			if !exists {
				continue
			}

			// Thử khớp action cụ thể và action "any"
			var actionsToTry [2]uint64
			actionsToTry[0] = actionHash
			actionsToTry[1] = anyHash
			for _, actHash := range actionsToTry {
				if actNode, exists := resNode.Actions[actHash]; exists {
					*buf = append(*buf, actNode.Policies...)
				}
			}
		}
	}
}

// ensureAnyInto kiểm tra xem "any" đã có trong list chưa.
// Nếu chưa, sao chép toàn bộ vào scratch buffer và thêm "any" — không tạo heap allocation.
// scratch phải đủ lớn (ít nhất len(list)+1).
func ensureAnyInto(list []string, scratch *[8]string) []string {
	for _, item := range list {
		if item == "any" {
			return list // Đã có "any" — trả về gốc, không cần copy.
		}
	}
	// Sao chép vào scratch (stack) và thêm "any".
	n := copy(scratch[:], list)
	if n < len(*scratch) {
		scratch[n] = "any"
		return scratch[:n+1]
	}
	// Fallback nếu list quá dài (> 7 phần tử) — heap allocation nhưng rất hiếm.
	result := make([]string, len(list)+1)
	copy(result, list)
	result[len(list)] = "any"
	return result
}
