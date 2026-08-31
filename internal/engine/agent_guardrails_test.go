package engine

import (
	"context"
	"testing"

	"standalone-policy-engine/internal/parser"
)

func TestAIAgentGuardrails_ToolAuthorization(t *testing.T) {
	eng := NewEngine()
	compiler := parser.NewCompiler()

	// 1. Luật Cho phép AI Agent chạy các tool thông thường
	permitDSL := `permit(
		principal == role:"ai_agent",
		action == action:EXECUTE,
		resource == any
	) when {
		context.tool_risk == "LOW"
	};`

	// 2. Luật Cấm tuyệt đối nếu tool có rủi ro tài chính hoặc delegation depth vượt quá 3
	forbidDSL := `forbid(
		principal == role:"ai_agent",
		action == action:EXECUTE,
		resource == any
	) when {
		context.tool_risk == "HIGH" || context.delegation_depth > 3
	};`

	l1 := parser.NewLexer(permitDSL)
	p1 := parser.NewParser(l1)
	nodes1 := p1.Parse()
	nodes1[0].ID = "PERMIT-LOW-RISK-TOOLS"
	c1, _ := compiler.Compile(nodes1[0])

	l2 := parser.NewLexer(forbidDSL)
	p2 := parser.NewParser(l2)
	nodes2 := p2.Parse()
	nodes2[0].ID = "FORBID-HIGH-RISK-TOOLS"
	c2, _ := compiler.Compile(nodes2[0])

	_ = eng.UpdateTenantPolicies("tenant-ai", []*parser.PolicyNode{c1, c2}, nil)

	ctx := context.Background()

	// Kịch bản 1: AI Agent gọi tool đọc thông tin (Low Risk, depth = 1) -> ALLOW
	res1 := eng.CheckPermission(ctx, "tenant-ai", "role:ai_agent", "EXECUTE", "tool:web_search", map[string]string{
		"tool_risk":        "LOW",
		"delegation_depth": "1",
	})
	if res1.Decision != DecisionAllow {
		t.Fatalf("Mong đợi ALLOW cho low risk tool, thực tế: %v (%s)", res1.Decision, res1.Reason)
	}

	// Kịch bản 2: AI Agent tự động gọi tool chuyển tiền (High Risk) -> DENY
	res2 := eng.CheckPermission(ctx, "tenant-ai", "role:ai_agent", "EXECUTE", "tool:transfer_funds", map[string]string{
		"tool_risk":        "HIGH",
		"delegation_depth": "1",
	})
	if res2.Decision != DecisionDeny {
		t.Fatalf("Mong đợi DENY cho high risk tool, thực tế: %v", res2.Decision)
	}

	// Gắn thêm Obligation vào quyết định DENY cho client
	if res2.Decision == DecisionDeny && res2.Reason == ReasonDenyForbid {
		res2.Obligations = []Obligation{
			{
				Type:    ObligationTypeRequireApproval,
				Message: "Yêu cầu phê duyệt từ cấp quản lý trước khi thực thi công cụ rủi ro cao",
				Payload: map[string]string{
					"tool_name":  "transfer_funds",
					"risk_level": "HIGH",
				},
			},
		}
	}
	if len(res2.Obligations) != 1 || res2.Obligations[0].Type != ObligationTypeRequireApproval {
		t.Errorf("Mong đợi Obligation REQUIRE_HUMAN_APPROVAL, thực tế: %v", res2.Obligations)
	}

	// Kịch bản 3: AI Agent vượt quá độ sâu ủy quyền (Delegation Depth = 5) -> DENY (Chống leo thang đặc quyền)
	res3 := eng.CheckPermission(ctx, "tenant-ai", "role:ai_agent", "EXECUTE", "tool:web_search", map[string]string{
		"tool_risk":        "LOW",
		"delegation_depth": "5",
	})
	if res3.Decision != DecisionDeny {
		t.Fatalf("Mong đợi DENY do delegation_depth > 3, thực tế: %v", res3.Decision)
	}
}
