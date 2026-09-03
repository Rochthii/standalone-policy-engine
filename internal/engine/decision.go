package engine

import (
	"context"
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

// Obligation đại diện cho một nghĩa vụ hoặc rào chắn bắt buộc mà PEP/Caller phải thực thi
// (ví dụ: yêu cầu con người phê duyệt, che giấu dữ liệu nhạy cảm, ghi log kiểm toán mở rộng).
type Obligation struct {
	Type    string            `json:"type"` // REQUIRE_HUMAN_APPROVAL, MASK_ATTRIBUTES, AUDIT_SENSITIVE_TOOL_CALL
	Message string            `json:"message"`
	Payload map[string]string `json:"payload,omitempty"`
}

const (
	ObligationTypeRequireApproval = "REQUIRE_HUMAN_APPROVAL"
	ObligationTypeAuditSensitive  = "AUDIT_SENSITIVE_TOOL_CALL"
	ObligationTypeMaskAttributes  = "MASK_ATTRIBUTES"
)

// DecisionResult chứa thông tin quyết định phân quyền cuối cùng.
type DecisionResult struct {
	Decision Decision
	Reason   string
	// Explanations chứa danh sách ID các chính sách trực tiếp dẫn đến quyết định này.
	Explanations []string
	// Obligations chứa danh sách các nghĩa vụ/rào chắn đính kèm (chuẩn NIST/OWASP cho AI Guardrails).
	Obligations []Obligation
}

var (
	emptyExplanations = []string{}
	emptyObligations  = []Obligation{}
)

const (
	ReasonDenyCanceled       = "Yêu cầu bị hủy hoặc hết thời gian chờ"
	ReasonDenyTenantNotFound = "Không tìm thấy tập chính sách cho Tenant"
	ReasonDenyNoMatch        = "Không có chính sách nào khớp với phạm vi yêu cầu"
	ReasonDenyForbid         = "Yêu cầu bị từ chối bởi luật cấm tường minh"
	ReasonAllowPermit        = "Yêu cầu được chấp thuận bởi luật cho phép"
	ReasonDenyDefault        = "Không tìm thấy luật cho phép nào thỏa mãn điều kiện"
)

// CheckPermission thực hiện tra cứu chỉ mục Trie trên RAM, đánh giá các biểu thức AST
// và đưa ra quyết định phân quyền cuối cùng dựa trên các quy tắc:
//  1. Deny-by-Default (Mặc định cấm)
//  2. Forbid Overrides (Luật cấm ghi đè luật cho phép)
func (e *Engine) CheckPermission(ctx context.Context, tenantID, subject, action, resource string, context map[string]string) DecisionResult {
	if err := ctx.Err(); err != nil {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       ReasonDenyCanceled,
			Explanations: emptyExplanations,
		}
	}

	// 1. Lấy cây Trie của Tenant
	trie, exists := e.GetTenantTrie(tenantID)
	if !exists {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       ReasonDenyTenantNotFound,
			Explanations: emptyExplanations,
		}
	}

	return evaluatePermission(ctx, trie, subject, action, resource, context)
}

func normalizeAction(act string) string {
	if !strings.HasPrefix(act, "action:") {
		return "action:" + act
	}
	return act
}

// CheckPermission là hàm mức package thực hiện quyết định phân quyền trực tiếp
// từ một TrieRoot cho trước (dùng bởi engine_gc.go và simulator).
func CheckPermission(ctx context.Context, trie *TrieRoot, subject, action, resource string, ctxMap map[string]string) DecisionResult {
	if err := ctx.Err(); err != nil {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       ReasonDenyCanceled,
			Explanations: emptyExplanations,
		}
	}

	if trie == nil {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       ReasonDenyTenantNotFound,
			Explanations: emptyExplanations,
		}
	}

	return evaluatePermission(ctx, trie, subject, action, resource, ctxMap)
}

func evaluatePermission(ctx context.Context, trie *TrieRoot, subject, action, resource string, context map[string]string) DecisionResult {
	// 1. Thu thập danh tính Subject (Zero-Allocation stack scratch)
	var subScratch [16]string
	subjects := trie.RoleDAG.GetInheritedRolesInto(subject, &subScratch)

	var resScratch [1]string
	resScratch[0] = resource
	resources := resScratch[:]
	actKey := normalizeAction(action)

	// 2. Tra cứu nhanh trên Trie bằng Buffer Pool
	policyBuf := GetPolicySlice()
	defer ReturnPolicySlice(policyBuf)
	trie.LookupPoliciesInto(policyBuf, subjects, resources, actKey)

	matchedPolicies := *policyBuf
	if len(matchedPolicies) == 0 {
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       ReasonDenyNoMatch,
			Explanations: emptyExplanations,
		}
	}

	// 3. Khởi tạo ngữ cảnh đánh giá từ sync.Pool
	evalCtx := GetEvalContext(subject, action, resource, context, trie.RoleDAG)
	defer evalCtx.Release()

	var firstForbidPolicy *parser.PolicyNode
	var firstPermitPolicy *parser.PolicyNode

	// 4. Đánh giá từng chính sách khớp
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
				firstForbidPolicy = policy
				break // Forbid Overrides: ngắt ngay lập tức (Short-Circuit)
			} else if policy.Effect == parser.EffectPermit && firstPermitPolicy == nil {
				firstPermitPolicy = policy
			}
		}
	}

	// 5. Áp dụng bảng chân trị quyết định
	if firstForbidPolicy != nil {
		exps := firstForbidPolicy.ExplanationList
		if len(exps) == 0 {
			exps = []string{firstForbidPolicy.ID}
		}
		return DecisionResult{
			Decision:     DecisionDeny,
			Reason:       ReasonDenyForbid,
			Explanations: exps,
		}
	}

	if firstPermitPolicy != nil {
		exps := firstPermitPolicy.ExplanationList
		if len(exps) == 0 {
			exps = []string{firstPermitPolicy.ID}
		}
		return DecisionResult{
			Decision:     DecisionAllow,
			Reason:       ReasonAllowPermit,
			Explanations: exps,
		}
	}

	// Mặc định cấm (Deny-by-Default)
	return DecisionResult{
		Decision:     DecisionDeny,
		Reason:       ReasonDenyDefault,
		Explanations: emptyExplanations,
	}
}
