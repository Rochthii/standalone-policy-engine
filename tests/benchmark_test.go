package tests

import (
	"context"
	"fmt"
	"testing"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
)
func TestDummy(t *testing.T) {}


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
	reqCtxWithCompromised := map[string]string{
		"ip_address":    "192.168.1.45",
		"request_time":  "12:00:00Z",
		"device_status": "compromised",
	}

	// Pre-generate subjects and resources để loại trừ chi phí fmt.Sprintf của benchmark framework
	subList := make([]string, 1000)
	resList := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		subList[i] = fmt.Sprintf("user:user_%d", i)
		resList[i] = fmt.Sprintf("file:doc_%d", i)
	}

	b.ResetTimer() // Loại bỏ thời gian biên dịch chính sách ra khỏi benchmark

	// 2. Chạy vòng lặp đo đạc thời gian quyết định
	for i := 0; i < b.N; i++ {
		userID := i % 1000
		subject := subList[userID]
		resource := resList[userID]

		res := eng.CheckPermission(context.Background(), tenantID, subject, "action:READ", resource, reqCtx)
		
		// Đo lường kết quả để tránh trình biên dịch Go optimize bỏ qua vòng lặp
		if userID < 990 {
			if res.Decision != engine.DecisionAllow {
				b.Fatalf("Mong đợi ALLOW cho user %s, thực tế: %s", subject, res.Reason)
			}
		} else {
			resCompromised := eng.CheckPermission(context.Background(), tenantID, subject, "action:READ", resource, reqCtxWithCompromised)
			if resCompromised.Decision != engine.DecisionDeny {
				b.Fatalf("Mong đợi DENY cho user bị compromised %s, thực tế: ALLOW", subject)
			}
		}
	}
}

// BenchmarkConcurrentLoad đo lường hiệu năng của PDP Engine dưới tải truy cập đồng thời lớn (Concurrent Load).
func BenchmarkConcurrentLoad(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()

	tenantID := "tenant-benchmark-conc"
	policies := make([]*parser.PolicyNode, 1000)
	for i := 0; i < 1000; i++ {
		dsl := fmt.Sprintf(`permit(principal == user:"user_%d", action == action:READ, resource == file:"doc_%d")
        when {
            context.ip_address in "192.168.1.0/24"
        };`, i, i)
		policies[i] = compileHelper(b, compiler, fmt.Sprintf("P-%d", i), dsl)
	}

	err := eng.UpdateTenantPolicies(tenantID, policies, nil)
	if err != nil {
		b.Fatalf("UpdateTenantPolicies failed: %v", err)
	}

	reqCtx := map[string]string{
		"ip_address": "192.168.1.50",
	}

	subList := make([]string, 1000)
	resList := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		subList[i] = fmt.Sprintf("user:user_%d", i)
		resList[i] = fmt.Sprintf("file:doc_%d", i)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			userID := idx % 1000
			idx++
			subject := subList[userID]
			resource := resList[userID]
			res := eng.CheckPermission(context.Background(), tenantID, subject, "action:READ", resource, reqCtx)
			if res.Decision != engine.DecisionAllow {
				b.Fatalf("Mong đợi ALLOW cho user %s, thực tế: %v", subject, res.Decision)
			}
		}
	})
}

// BenchmarkUltraExtreme_DeepDAG_HeavyABAC đo lường tình huống "ác mộng" nhất (Worst-Case Scenario):
// 1. Cây phân cấp vai trò cực sâu: 10 cấp thừa kế (user -> level_1 -> ... -> level_10 -> super_admin)
// 2. Bộ dữ liệu lớn: 5,000 chính sách trong Trie
// 3. Biểu thức AST ABAC siêu nặng (10+ điều kiện kết hợp: CIDR IP, Integer, Time, Strings, Device status)
// 4. Đường dẫn tra cứu xấu nhất: miss Subject cụ thể -> truy vết qua 10 tổ tiên DAG -> duyệt candidate policies -> khớp Forbid ở điều kiện cuối cùng.
func BenchmarkUltraExtreme_DeepDAG_HeavyABAC(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "tenant-hell-mode"

	// 1. Tạo chuỗi kế thừa 10 cấp
	inheritances := [][2]string{
		{"user:agent_007", "role:level_1"},
		{"role:level_1", "role:level_2"},
		{"role:level_2", "role:level_3"},
		{"role:level_3", "role:level_4"},
		{"role:level_4", "role:level_5"},
		{"role:level_5", "role:level_6"},
		{"role:level_6", "role:level_7"},
		{"role:level_7", "role:level_8"},
		{"role:level_8", "role:level_9"},
		{"role:level_9", "role:level_10"},
		{"role:level_10", "role:enterprise_god_admin"},
	}

	// 2. Tạo 5,000 chính sách rải khắp các node
	totalPolicies := 5000
	policies := make([]*parser.PolicyNode, totalPolicies)

	for i := 0; i < totalPolicies-2; i++ {
		dsl := fmt.Sprintf(`permit(principal == user:"decoy_user_%d", action == action:READ, resource == file:"doc_%d")
		when {
			context.ip_address in "10.0.0.0/8" &&
			context.amount > "100" &&
			context.request_time >= "06:00:00Z"
		};`, i, i)
		policies[i] = compileHelper(b, compiler, fmt.Sprintf("P-DECOY-%d", i), dsl)
	}

	// Chính sách Permit gắn ở đỉnh ngọn DAG (role:enterprise_god_admin) với AST siêu nặng 10 điều kiện
	heavyPermitDSL := `permit(principal == role:enterprise_god_admin, action == action:EXECUTE, resource == resource:nuke_codes)
	when {
		context.ip_address in "10.240.0.0/16" &&
		context.amount > 500000 &&
		context.risk_score <= 15 &&
		context.branch == "SG-HQ" &&
		context.device == "corporate_hardened" &&
		context.mfa_verified == "true" &&
		context.clearance == "top_secret" &&
		context.request_time >= "08:00:00Z" &&
		context.request_time <= "18:00:00Z"
	};`
	policies[totalPolicies-2] = compileHelper(b, compiler, "P-HELL-PERMIT", heavyPermitDSL)

	// Chính sách Forbid toàn cục (sẽ quét qua nếu device bị compromised)
	forbidDSL := `forbid(principal == any, action == action:EXECUTE, resource == resource:nuke_codes)
	when {
		context.device_compromised == "true"
	};`
	policies[totalPolicies-1] = compileHelper(b, compiler, "P-HELL-FORBID", forbidDSL)

	err := eng.UpdateTenantPolicies(tenantID, policies, inheritances)
	if err != nil {
		b.Fatalf("Update policies failed: %v", err)
	}

	// Ngữ cảnh thỏa mãn Permit
	validContext := map[string]string{
		"ip_address":         "10.240.5.100",
		"amount":             "1000000",
		"risk_score":         "10",
		"branch":             "SG-HQ",
		"device":             "corporate_hardened",
		"mfa_verified":       "true",
		"clearance":          "top_secret",
		"request_time":       "14:30:00Z",
		"device_compromised": "false",
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		res := eng.CheckPermission(ctx, tenantID, "user:agent_007", "action:EXECUTE", "resource:nuke_codes", validContext)
		if res.Decision != engine.DecisionAllow {
			b.Fatalf("Kỳ vọng ALLOW, thực tế: %v, lý do: %s", res.Decision, res.Reason)
		}
	}
}

// BenchmarkUltraExtreme_10kPolicies_ConcurrentContention đo lường 10,000 chính sách chịu tải đa luồng cực đại.
func BenchmarkUltraExtreme_10kPolicies_ConcurrentContention(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "tenant-10k-stress"

	totalPolicies := 10000
	policies := make([]*parser.PolicyNode, totalPolicies)

	for i := 0; i < totalPolicies; i++ {
		dsl := fmt.Sprintf(`permit(principal == user:"user_%d", action == action:READ, resource == file:"doc_%d")
		when {
			context.ip_address in "192.168.0.0/16" &&
			context.security_level >= "3"
		};`, i, i)
		policies[i] = compileHelper(b, compiler, fmt.Sprintf("P-10K-%d", i), dsl)
	}

	err := eng.UpdateTenantPolicies(tenantID, policies, nil)
	if err != nil {
		b.Fatalf("UpdateTenantPolicies failed: %v", err)
	}

	reqCtx := map[string]string{
		"ip_address":     "192.168.1.100",
		"security_level": "5",
	}

	subList := make([]string, totalPolicies)
	resList := make([]string, totalPolicies)
	for i := 0; i < totalPolicies; i++ {
		subList[i] = fmt.Sprintf("user:user_%d", i)
		resList[i] = fmt.Sprintf("file:doc_%d", i)
	}

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			userID := idx % totalPolicies
			idx++
			res := eng.CheckPermission(ctx, tenantID, subList[userID], "action:READ", resList[userID], reqCtx)
			if res.Decision != engine.DecisionAllow {
				b.Fatalf("Kỳ vọng ALLOW cho user_%d", userID)
			}
		}
	})
}

// Helper biên dịch chính sách
func compileHelper(t testing.TB, c *parser.Compiler, id string, dsl string) *parser.PolicyNode {
	l := parser.NewLexer(dsl)
	p := parser.NewParser(l)
	nodes := p.Parse()

	if len(p.Errors()) > 0 {
		t.Fatalf("Lỗi parse DSL [%s]: %v", id, p.Errors())
	}

	nodes[0].ID = id
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("Lỗi compile DSL [%s]: %v", id, err)
	}

	return compiled
}
