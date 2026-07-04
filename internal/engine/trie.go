package engine

import (
	"fmt"
	"standalone-policy-engine/internal/parser"
)

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
// Để hỗ trợ quan hệ kế thừa vai trò (RBAC) và wildcard, hàm nhận vào:
//   - subjects: danh sách các danh tính khớp của subject (ví dụ: ["user:alice", "role:operator", "any"])
//   - resources: danh sách danh tính của resource (ví dụ: ["file:report.pdf", "any"])
//   - action: hành động yêu cầu (ví dụ: "action:READ" hoặc "any")
func (t *TrieRoot) LookupPolicies(subjects []string, resources []string, action string) []*parser.PolicyNode {
	matched := make([]*parser.PolicyNode, 0)

	// 1. Luôn gộp các chính sách toàn cục (Global Rules Partition)
	matched = append(matched, t.GlobalPolicies...)

	// Đảm bảo "any" luôn nằm trong danh sách subjects và resources để so khớp wildcard
	subjects = ensureAny(subjects)
	resources = ensureAny(resources)

	// 2. Tra cứu trên cây Trie theo tất cả các tổ hợp danh tính hợp lệ
	for _, subKey := range subjects {
		subNode, exists := t.Subjects[subKey]
		if !exists {
			continue
		}

		for _, resKey := range resources {
			resNode, exists := subNode.Resources[resKey]
			if !exists {
				continue
			}

			// Thử khớp action cụ thể và action "any" (wildcard)
			actionsToTry := []string{action, "any"}
			for _, actKey := range actionsToTry {
				if actNode, exists := resNode.Actions[actKey]; exists {
					matched = append(matched, actNode.Policies...)
				}
			}
		}
	}

	return matched
}

func ensureAny(list []string) []string {
	hasAny := false
	for _, item := range list {
		if item == "any" {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return append(list, "any")
	}
	return list
}
