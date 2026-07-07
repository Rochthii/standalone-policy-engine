package engine

import (
	"fmt"
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

	// Subjects map các định danh principal (ví dụ: "user:alice", "role:admin", hoặc "any")
	// trỏ tới danh sách các tài nguyên tương ứng.
	Subjects map[string]*SubjectNode

	// GlobalPolicies chứa các chính sách toàn cục (Global Rules Partition)
	// nơi cả principal và resource đều được thiết lập là 'any' (wildcard kép).
	GlobalPolicies []*parser.PolicyNode

	// RoleDAG quản lý phân cấp vai trò cho riêng Tenant này.
	RoleDAG *RoleDAG
}

// SubjectNode đại diện cho phân cấp Subject trong Trie.
type SubjectNode struct {
	// Resources map các định danh resource (ví dụ: "file:report.pdf", "folder:finance", hoặc "any")
	Resources map[string]*ResourceNode
}

// ResourceNode đại diện cho phân cấp Resource trong Trie.
type ResourceNode struct {
	// Actions map các định danh action (ví dụ: "action:READ", "action:DELETE", hoặc "any")
	Actions map[string]*ActionNode
}

// ActionNode đại diện cho phân cấp Action trong Trie (nút lá chứa chính sách).
type ActionNode struct {
	Policies []*parser.PolicyNode
}

// NewTrieRoot tạo mới một instance TrieRoot cho một Tenant.
func NewTrieRoot(tenantID string) *TrieRoot {
	return &TrieRoot{
		TenantID:       tenantID,
		Subjects:       make(map[string]*SubjectNode),
		GlobalPolicies: make([]*parser.PolicyNode, 0),
		RoleDAG:        NewRoleDAG(),
	}
}

// buildKey chuyển đổi ScopeNode tĩnh thành định dạng chuỗi khóa để lưu trữ trong Trie.
// Ví dụ: user:"alice" -> "user:alice"
// Ví dụ: any -> "any"
func buildKey(scope *parser.ScopeNode) string {
	if scope == nil || scope.Operator == parser.ScopeOpAny {
		return "any"
	}
	return fmt.Sprintf("%s:%s", scope.EntityType, scope.EntityID)
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

	// 2. Đi theo luồng phân cấp: Subject -> Resource -> Action
	subKey := buildKey(policy.Principal)
	resKey := buildKey(policy.Resource)
	actKey := buildKey(policy.Action)

	// Lấy hoặc tạo SubjectNode
	subNode, exists := t.Subjects[subKey]
	if !exists {
		subNode = &SubjectNode{Resources: make(map[string]*ResourceNode)}
		t.Subjects[subKey] = subNode
	}

	// Lấy hoặc tạo ResourceNode
	resNode, exists := subNode.Resources[resKey]
	if !exists {
		resNode = &ResourceNode{Actions: make(map[string]*ActionNode)}
		subNode.Resources[resKey] = resNode
	}

	// Lấy hoặc tạo ActionNode
	actNode, exists := resNode.Actions[actKey]
	if !exists {
		actNode = &ActionNode{Policies: make([]*parser.PolicyNode, 0)}
		resNode.Actions[actKey] = actNode
	}

	// Thêm chính sách vào danh sách nút lá
	actNode.Policies = append(actNode.Policies, policy)
}

// LookupPolicies tra cứu và thu thập tất cả các chính sách khớp với ngữ cảnh tĩnh của yêu cầu.
// Đây là API tương thích ngược, phân bổ slice mới mỗi lần.
// Với hiệu năng cao hơn, dùng LookupPoliciesInto kết hợp GetPolicySlice/ReturnPolicySlice.
//
//   - subjects: danh sách các danh tính khớp của subject (ví dụ: ["user:alice", "role:operator", "any"])
//   - resources: danh sách danh tính của resource (ví dụ: ["file:report.pdf", "any"])
//   - action: hành động yêu cầu (ví dụ: "action:READ" hoặc "any")
func (t *TrieRoot) LookupPolicies(subjects []string, resources []string, action string) []*parser.PolicyNode {
	buf := GetPolicySlice()
	t.LookupPoliciesInto(buf, subjects, resources, action)
	// Sao chép ra slice độc lập trước khi trả về — caller không cần quản lý vòng đời pool.
	result := make([]*parser.PolicyNode, len(*buf))
	copy(result, *buf)
	ReturnPolicySlice(buf)
	return result
}

// LookupPoliciesInto là phiên bản zero-allocation của LookupPolicies.
// Kết quả được append vào buf thay vì tạo slice mới.
// Caller phải lấy buf bằng GetPolicySlice() và trả về bằng ReturnPolicySlice() sau khi dùng xong.
func (t *TrieRoot) LookupPoliciesInto(buf *[]*parser.PolicyNode, subjects []string, resources []string, action string) {
	// 1. Luôn gộp các chính sách toàn cục (Global Rules Partition)
	*buf = append(*buf, t.GlobalPolicies...)

	// Đảm bảo "any" luôn nằm trong danh sách để so khớp wildcard.
	// Dùng scratch buffer trên stack để tránh heap allocation từ append.
	var subScratch [8]string
	var resScratch [8]string
	subjectsWithAny := ensureAnyInto(subjects, &subScratch)
	resourcesWithAny := ensureAnyInto(resources, &resScratch)

	// 2. Tra cứu trên cây Trie theo tất cả các tổ hợp danh tính hợp lệ
	for _, subKey := range subjectsWithAny {
		subNode, exists := t.Subjects[subKey]
		if !exists {
			continue
		}

		for _, resKey := range resourcesWithAny {
			resNode, exists := subNode.Resources[resKey]
			if !exists {
				continue
			}

			// Stack-allocated array — không heap, không GC pressure.
			var actionsToTry [2]string
			actionsToTry[0] = action
			actionsToTry[1] = "any"
			for _, actKey := range actionsToTry {
				if actNode, exists := resNode.Actions[actKey]; exists {
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
