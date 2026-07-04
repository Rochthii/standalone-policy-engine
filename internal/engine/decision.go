package engine

import (
	"fmt"
	"standalone-policy-engine/internal/parser"
	"strings"
)

// Decision định nghĩa kiểu quyết định phân quyền.
type Decision int32

const (
	DecisionDeny  Decision = 0
	DecisionAllow Decision = 1
)

// String trả về chuỗi đại diện của Decision.
func (d Decision) String() string {
	if d == DecisionAllow {
		return "ALLOW"
	}
	return "DENY"
}

// DecisionResult chứa thông tin quyết định phân quyền cuối cùng.
type DecisionResult struct {
	Decision Decision
	Reason   string
	// Explanations chứa danh sách ID các chính sách trực tiếp dẫn đến quyết định này.
	Explanations []string
}

// CheckPermission thực hiện tra cứu chỉ mục Trie trên RAM, đánh giá các biểu thức AST
// và đưa ra quyết định phân quyền cuối cùng dựa trên các quy tắc:
//  1. Deny-by-Default (Mặc định cấm)
//  2. Forbid Overrides (Luật cấm ghi đè luật cho phép)
func (e *Engine) CheckPermission(tenantID, subject, action, resource string, context map[string]string) DecisionResult {
	// 1. Lấy cây Trie của Tenant
	trie, exists := e.GetTenantTrie(tenantID)
	if !exists {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       "Không tìm thấy tập chính sách cho Tenant",
			Explanations: []string{},
		}
	}

	// 2. Thu thập danh tính của Subject dựa trên đồ thị phân cấp vai trò
	subjects := trie.RoleDAG.GetInheritedRoles(subject)

	// Chuẩn hóa resource và action
	resources := []string{resource}
	actKey := normalizeAction(action)

	// 3. Tra cứu nhanh trên Trie để lấy các chính sách khớp tĩnh
	matchedPolicies := trie.LookupPolicies(subjects, resources, actKey)
	if len(matchedPolicies) == 0 {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       "Không có chính sách nào khớp với phạm vi yêu cầu",
			Explanations: []string{},
		}
	}

	// 4. Khởi tạo ngữ cảnh đánh giá từ sync.Pool
	evalCtx := GetEvalContext(subject, action, resource, context, trie.RoleDAG)
	defer evalCtx.Release()

	forbidMatched := make([]string, 0)
	permitMatched := make([]string, 0)

	// 5. Đánh giá từng chính sách khớp
	for _, policy := range matchedPolicies {
		// Đánh giá biểu thức logic
		val, err := Evaluate(policy.Condition, evalCtx)
		
		// Xử lý logic Condition
		isConditionSatisfied := false
		if err == nil && val.ValType == parser.ValueTypeBool {
			// unless đảo ngược giá trị logic của when
			if policy.IsUnless {
				isConditionSatisfied = !val.BoolVal
			} else {
				isConditionSatisfied = val.BoolVal
			}
		}

		if isConditionSatisfied {
			if policy.Effect == parser.EffectForbid {
				forbidMatched = append(forbidMatched, policy.ID)
			} else if policy.Effect == parser.EffectPermit {
				permitMatched = append(permitMatched, policy.ID)
			}
		}
	}

	// 6. Áp dụng bảng chân trị quyết định
	// Forbid Overrides: Nếu có bất kỳ luật cấm nào thỏa mãn, trả về DENY ngay lập tức
	if len(forbidMatched) > 0 {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       fmt.Sprintf("Yêu cầu bị từ chối bởi luật cấm tường minh: %s", strings.Join(forbidMatched, ", ")),
			Explanations: forbidMatched,
		}
	}

	// Nếu không có luật cấm, và có ít nhất một luật cho phép thỏa mãn
	if len(permitMatched) > 0 {
		return DecisionResult{
			Decision:     DecisionAllow,
			Reason:       fmt.Sprintf("Yêu cầu được chấp thuận bởi luật: %s", strings.Join(permitMatched, ", ")),
			Explanations: permitMatched,
		}
	}

	// Mặc định cấm (Deny-by-Default)
	return DecisionResult{
		Decision:     DecisionDeny,
		Reason:       "Không tìm thấy luật cho phép nào thỏa mãn điều kiện",
		Explanations: []string{},
	}
}

func normalizeAction(act string) string {
	if !strings.HasPrefix(act, "action:") {
		return "action:" + act
	}
	return act
}

// CheckPermission là hàm mức package thực hiện quyết định phân quyền trực tiếp
// từ một TrieRoot cho trước (dùng bởi engine_gc.go và simulator).
func CheckPermission(trie *TrieRoot, subject, action, resource string, ctxMap map[string]string) DecisionResult {
	// Thu thập danh tính Subject
	subjects := trie.RoleDAG.GetInheritedRoles(subject)
	resources := []string{resource}
	actKey := normalizeAction(action)

	matchedPolicies := trie.LookupPolicies(subjects, resources, actKey)
	if len(matchedPolicies) == 0 {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       "Không có chính sách nào khớp với phạm vi yêu cầu",
			Explanations: []string{},
		}
	}

	evalCtx := GetEvalContext(subject, action, resource, ctxMap, trie.RoleDAG)
	defer evalCtx.Release()

	forbidMatched := make([]string, 0)
	permitMatched := make([]string, 0)

	for _, policy := range matchedPolicies {
		val, err := Evaluate(policy.Condition, evalCtx)
		isConditionSatisfied := false
		if err == nil && val.ValType == parser.ValueTypeBool {
			if policy.IsUnless {
				isConditionSatisfied = !val.BoolVal
			} else {
				isConditionSatisfied = val.BoolVal
			}
		}

		if isConditionSatisfied {
			if policy.Effect == parser.EffectForbid {
				forbidMatched = append(forbidMatched, policy.ID)
			} else if policy.Effect == parser.EffectPermit {
				permitMatched = append(permitMatched, policy.ID)
			}
		}
	}

	if len(forbidMatched) > 0 {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       fmt.Sprintf("Yêu cầu bị từ chối bởi luật cấm tường minh: %s", strings.Join(forbidMatched, ", ")),
			Explanations: forbidMatched,
		}
	}

	if len(permitMatched) > 0 {
		return DecisionResult{
			Decision:     DecisionAllow,
			Reason:       fmt.Sprintf("Yêu cầu được chấp thuận bởi luật: %s", strings.Join(permitMatched, ", ")),
			Explanations: permitMatched,
		}
	}

	return DecisionResult{
		Decision:     DecisionDeny,
		Reason:       "Không tìm thấy luật cho phép nào thỏa mãn điều kiện",
		Explanations: []string{},
	}
}

