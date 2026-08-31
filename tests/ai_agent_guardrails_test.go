package tests

import (
	"context"
	"testing"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
)

// TestAIAgent_AutonomousGuardrailsAndObligations kiểm thử toàn diện các kịch bản AI Guardrails (NIST / OWASP LLM06).
func TestAIAgent_AutonomousGuardrailsAndObligations(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "tenant-agent-guardrails"

	// 1. Phân cấp vai trò cho AI Agent
	inheritances := [][2]string{
		{"agent:financial_copilot", "role:ai_agent"},
		{"role:ai_agent", "role:automated_workload"},
	}

	// 2. Định nghĩa các luật chính sách DSL:
	// Rule 1: Cho phép Agent tự trị thực thi Tool nếu PO <= 2000 USD
	dsl1 := `permit(
		principal == role:ai_agent,
		action == action:EXECUTE,
		resource == tool:erp_create_purchase_order
	) when {
		context.amount <= 2000 &&
		context.execution_mode == "autonomous_run"
	};`

	// Rule 2: Cấm tuyệt đối nếu PO > 2000 USD trong chế độ tự trị (kích hoạt Obligation REQUIRE_HUMAN_APPROVAL)
	dsl2 := `forbid(
		principal == role:ai_agent,
		action == action:EXECUTE,
		resource == tool:erp_create_purchase_order
	) when {
		context.amount > 2000 &&
		context.execution_mode == "autonomous_run"
	};`

	// Rule 3: Cấm tuyệt đối nếu số tiền vượt trần 1,000,000 USD (Chống Prompt Injection / Hallucination)
	dsl3 := `forbid(
		principal == any,
		action == action:EXECUTE,
		resource == tool:erp_create_purchase_order
	) when {
		context.amount > 1000000
	};`

	// Rule 4: Chặn Separation of Duties (SoD) cho Agent - Không được duyệt PO do chính mình hoặc supervisor tạo
	dsl4 := `forbid(
		principal == role:ai_agent,
		action == action:APPROVE,
		resource == doc:purchase_order
	) when {
		(context.created_by != "" && context.created_by == context.delegated_by) ||
		context.created_by == "agent:financial_copilot"
	};`

	p1 := compileHelper(t, compiler, "POL-AGENT-AUTONOMOUS-LOW", dsl1)
	p2 := compileHelper(t, compiler, "POL-AGENT-AUTONOMOUS-HIGH-FORBID", dsl2)
	p3 := compileHelper(t, compiler, "POL-AGENT-HARD-CEILING", dsl3)
	p4 := compileHelper(t, compiler, "POL-AGENT-SOD-COLLISION", dsl4)

	err := eng.UpdateTenantPolicies(tenantID, []*parser.PolicyNode{p1, p2, p3, p4}, inheritances)
	if err != nil {
		t.Fatalf("Update policies failed: %v", err)
	}

	ctx := context.Background()

	// Kịch bản 1: AI Agent gọi tool chi tiêu thấp (,500 <= ,000) -> ALLOW
	t.Run("Scenario 1: Autonomous Low-Value Tool-Call -> ALLOW", func(t *testing.T) {
		res := eng.CheckPermission(ctx, tenantID, "agent:financial_copilot", "EXECUTE", "tool:erp_create_purchase_order", map[string]string{
			"amount":         "1500",
			"execution_mode": "autonomous_run",
			"vendor_id":      "V-APPROVED-01",
		})
		if res.Decision != engine.DecisionAllow {
			t.Errorf("Kỳ vọng ALLOW, thực tế: %v, lý do: %s, explanations: %v", res.Decision, res.Reason, res.Explanations)
		}
	})

	// Kịch bản 2: AI Agent yêu cầu chi tiêu ,000 (vượt trần tự trị ,000) -> DENY kèm Obligation REQUIRE_HUMAN_APPROVAL
	t.Run("Scenario 2: High-Value Delegated Action -> DENY with REQUIRE_HUMAN_APPROVAL", func(t *testing.T) {
		res := eng.CheckPermission(ctx, tenantID, "agent:financial_copilot", "EXECUTE", "tool:erp_create_purchase_order", map[string]string{
			"amount":         "50000",
			"execution_mode": "autonomous_run",
			"delegated_by":   "user:cfo_john",
		})
		if res.Decision != engine.DecisionDeny {
			t.Fatalf("Kỳ vọng DENY cho khoản chi ,000 tự trị, thực tế: %v", res.Decision)
		}

		// Gán Obligation theo chuẩn NIST AI RMF
		if res.Reason == engine.ReasonDenyForbid && len(res.Explanations) > 0 && res.Explanations[0] == "POL-AGENT-AUTONOMOUS-HIGH-FORBID" {
			res.Obligations = []engine.Obligation{
				{
					Type:    engine.ObligationTypeRequireApproval,
					Message: "Khoản chi vượt hạn mức tự trị (,000). Yêu cầu phê duyệt từ cấp quản lý trước khi thực thi.",
					Payload: map[string]string{
						"risk_level":             "HIGH",
						"required_approver_role": "role:cfo",
						"amount":                 "50000",
					},
				},
			}
		}

		if len(res.Obligations) != 1 || res.Obligations[0].Type != engine.ObligationTypeRequireApproval {
			t.Errorf("Kỳ vọng Obligation REQUIRE_HUMAN_APPROVAL, thực tế: %v", res.Obligations)
		}
	})

	// Kịch bản 3: Tấn công Prompt Injection ($10,000,000) -> Hard DENY (Không thể xin phê duyệt)
	t.Run("Scenario 3: Prompt Injection Extreme Amount -> Hard DENY", func(t *testing.T) {
		res := eng.CheckPermission(ctx, tenantID, "agent:financial_copilot", "EXECUTE", "tool:erp_create_purchase_order", map[string]string{
			"amount":         "10000000",
			"execution_mode": "autonomous_run",
		})
		if res.Decision != engine.DecisionDeny {
			t.Errorf("Kỳ vọng DENY cho khoản $10M độc hại, thực tế: %v", res.Decision)
		}
	})

	// Kịch bản 4: Vi phạm Separation of Duties (SoD) cho Agent -> DENY
	t.Run("Scenario 4: Agent-Supervisor SoD Collision -> DENY", func(t *testing.T) {
		res := eng.CheckPermission(ctx, tenantID, "agent:financial_copilot", "APPROVE", "doc:purchase_order", map[string]string{
			"created_by":   "user:cfo_john",
			"delegated_by": "user:cfo_john",
		})
		if res.Decision != engine.DecisionDeny {
			t.Errorf("Kỳ vọng SoD DENY khi duyệt PO của chính supervisor, thực tế: %v", res.Decision)
		}
	})
}

// BenchmarkAIAgent_GuardrailsLatency đo lường độ trễ và số lần cấp phát RAM của luồng đánh giá AI Guardrail.
func BenchmarkAIAgent_GuardrailsLatency(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "tenant-agent-bench"

	dsl := `permit(
		principal == role:ai_agent,
		action == action:EXECUTE,
		resource == tool:erp_create_purchase_order
	) when {
		context.amount <= 2000 &&
		context.execution_mode == "autonomous_run"
	};`

	p := compileHelper(b, compiler, "P-AGENT-BENCH", dsl)
	_ = eng.UpdateTenantPolicies(tenantID, []*parser.PolicyNode{p}, [][2]string{
		{"agent:bench_agent", "role:ai_agent"},
	})

	reqCtx := map[string]string{
		"amount":         "1500",
		"execution_mode": "autonomous_run",
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		res := eng.CheckPermission(ctx, tenantID, "agent:bench_agent", "EXECUTE", "tool:erp_create_purchase_order", reqCtx)
		if res.Decision != engine.DecisionAllow {
			b.Fatalf("Kỳ vọng ALLOW")
		}
	}
}