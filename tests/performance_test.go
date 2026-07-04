package tests

import (
	"fmt"
	"math/rand"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"testing"
)

// BenchmarkEvaluatorLatency đo lường độ trễ (Latency) trực tiếp của PDP Engine
// khi thực hiện quyết định phân quyền CheckPermission trên bộ nhớ RAM.
func BenchmarkEvaluatorLatency(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()

	// 1. Tạo tập dữ liệu 1000 chính sách ngẫu nhiên của Tenant
	tenantID := "tenant-benchmark"
	policies := make([]*parser.PolicyNode, 1000)
	
	// Tạo chính sách cho phép cụ thể
	for i := 0; i < 990; i++ {
		dsl := fmt.Sprintf(`permit(principal == user:"user_%d", action == action:READ, resource == file:"doc_%d")
        when {
            context.ip_address in "192.168.1.0/24" &&
            context.request_time >= "08:00:00Z" &&
            context.request_time <= "17:00:00Z"
        };`, i, i)
		policies[i] = compileHelper(b, compiler, fmt.Sprintf("P-%d", i), dsl)
	}

	// Tạo 10 chính sách cấm tường minh (Forbid)
	for i := 990; i < 1000; i++ {
		dsl := fmt.Sprintf(`forbid(principal == user:"user_%d", action == any, resource == any)
        when {
            context.device_status == "compromised"
        };`, i)
		policies[i] = compileHelper(b, compiler, fmt.Sprintf("P-%d", i), dsl)
	}

	// Phân cấp vai trò: user_100 kế thừa vai trò role:operator
	inheritances := [][2]string{
		{"user_100", "role:operator"},
	}

	err := eng.UpdateTenantPolicies(tenantID, policies, inheritances)
	if err != nil {
		b.Fatalf("Khởi tạo chính sách thất bại: %v", err)
	}

	// Chuẩn bị ngữ cảnh yêu cầu mẫu
	reqCtx := map[string]string{
		"ip_address":    "192.168.1.45",
		"request_time":  "12:00:00Z",
		"device_status": "secure",
	}

	b.ResetTimer() // Loại bỏ thời gian biên dịch chính sách ra khỏi benchmark

	// 2. Chạy vòng lặp đo đạc thời gian quyết định
	for i := 0; i < b.N; i++ {
		// So khớp ngẫu nhiên giữa các user để đo thời gian trung bình thực tế
		userID := rand.Intn(1000)
		subject := fmt.Sprintf("user_%d", userID)
		resource := fmt.Sprintf("file:doc_%d", userID)

		res := eng.CheckPermission(tenantID, subject, "READ", resource, reqCtx)
		
		// Đo lường kết quả để tránh trình biên dịch Go optimize bỏ qua vòng lặp
		if userID < 990 {
			if res.Decision != engine.DecisionAllow {
				b.Fatalf("Mong đợi ALLOW cho user %s, thực tế: %s", subject, res.Reason)
			}
		} else {
			// user bị cấm do luật forbid
			reqCtxWithCompromised := map[string]string{
				"ip_address":    "192.168.1.45",
				"request_time":  "12:00:00Z",
				"device_status": "compromised",
			}
			resCompromised := eng.CheckPermission(tenantID, subject, "READ", resource, reqCtxWithCompromised)
			if resCompromised.Decision != engine.DecisionDeny {
				b.Fatalf("Mong đợi DENY cho user bị compromised %s, thực tế: ALLOW", subject)
			}
		}
	}
}

// Helper biên dịch chính sách
func compileHelper(b *testing.B, c *parser.Compiler, id string, dsl string) *parser.PolicyNode {
	l := parser.NewLexer(dsl)
	p := parser.NewParser(l)
	nodes := p.Parse()

	if len(p.Errors()) > 0 {
		b.Fatalf("Lỗi parse DSL [%s]: %v", id, p.Errors())
	}

	nodes[0].ID = id
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		b.Fatalf("Lỗi compile DSL [%s]: %v", id, err)
	}

	return compiled
}
