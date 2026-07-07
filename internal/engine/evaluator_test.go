package engine

import (
	"context"
	"testing"

	"standalone-policy-engine/internal/parser"
)

// --- Helpers ---

func mustCompile(t *testing.T, id, dsl string) *parser.PolicyNode {
	t.Helper()
	c := parser.NewCompiler()
	l := parser.NewLexer(dsl)
	p := parser.NewParser(l)
	nodes := p.Parse()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse error [%s]: %v", id, p.Errors())
	}
	nodes[0].ID = id
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("compile error [%s]: %v", id, err)
	}
	return compiled
}

func newEngineWith(t *testing.T, tenantID string, policies []*parser.PolicyNode, inheritances [][2]string) *Engine {
	t.Helper()
	eng := NewEngine()
	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies failed: %v", err)
	}
	return eng
}

// --- TestEvaluator_IPCIDRMatching ---

// Kiểm tra logic CIDR matching: IP nằm trong dải được phép và IP ngoài dải.
func TestEvaluator_IPCIDRMatching(t *testing.T) {
	const tenantID = "tenant-ip-test"

	policy := mustCompile(t, "P-IP", `permit(principal == any, action == any, resource == any)
when {
    context.ip_address in "10.0.0.0/8"
};`)

	eng := newEngineWith(t, tenantID, []*parser.PolicyNode{policy}, nil)

	tests := []struct {
		name     string
		ip       string
		expected Decision
	}{
		{
			name:     "IP nằm trong CIDR 10.0.0.0/8 → ALLOW",
			ip:       "10.1.2.3",
			expected: DecisionAllow,
		},
		{
			name:     "IP boundary đầu dải → ALLOW",
			ip:       "10.0.0.1",
			expected: DecisionAllow,
		},
		{
			name:     "IP ngoài CIDR (192.168.x) → DENY",
			ip:       "192.168.1.1",
			expected: DecisionDeny,
		},
		{
			name:     "IP ngoài CIDR (172.16.x) → DENY",
			ip:       "172.16.0.1",
			expected: DecisionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := eng.CheckPermission(context.Background(), tenantID, "user:any", "READ", "file:any", map[string]string{
				"ip_address": tt.ip,
			})
			if res.Decision != tt.expected {
				t.Errorf("IP=%s: mong đợi %v, thực tế %v (lý do: %s)", tt.ip, tt.expected, res.Decision, res.Reason)
			}
		})
	}
}

// --- TestEvaluator_DateTimeComparison ---

// Kiểm tra logic so sánh thời gian: trong giờ làm việc và ngoài giờ.
func TestEvaluator_DateTimeComparison(t *testing.T) {
	const tenantID = "tenant-time-test"

	policy := mustCompile(t, "P-TIME", `permit(principal == any, action == any, resource == any)
when {
    context.request_time >= "08:00:00Z" &&
    context.request_time <= "18:00:00Z"
};`)

	eng := newEngineWith(t, tenantID, []*parser.PolicyNode{policy}, nil)

	tests := []struct {
		name     string
		reqTime  string
		expected Decision
	}{
		{
			name:     "Trong giờ làm việc 12:00 → ALLOW",
			reqTime:  "12:00:00Z",
			expected: DecisionAllow,
		},
		{
			name:     "Đúng mốc mở cửa 08:00 → ALLOW",
			reqTime:  "08:00:00Z",
			expected: DecisionAllow,
		},
		{
			name:     "Đúng mốc đóng cửa 18:00 → ALLOW",
			reqTime:  "18:00:00Z",
			expected: DecisionAllow,
		},
		{
			name:     "Ngoài giờ 02:00 (ban đêm) → DENY",
			reqTime:  "02:00:00Z",
			expected: DecisionDeny,
		},
		{
			name:     "Ngoài giờ 23:59 (tối muộn) → DENY",
			reqTime:  "23:59:00Z",
			expected: DecisionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := eng.CheckPermission(context.Background(), tenantID, "user:any", "READ", "file:any", map[string]string{
				"request_time": tt.reqTime,
			})
			if res.Decision != tt.expected {
				t.Errorf("time=%s: mong đợi %v, thực tế %v (lý do: %s)", tt.reqTime, tt.expected, res.Decision, res.Reason)
			}
		})
	}
}

// --- TestEvaluator_MissingAttributeFailClosed ---

// Nguyên tắc fail-closed: thuộc tính thiếu trong context phải dẫn đến DENY,
// không được panic hoặc default về ALLOW.
func TestEvaluator_MissingAttributeFailClosed(t *testing.T) {
	const tenantID = "tenant-failclosed-test"

	// Policy yêu cầu cả ip_address VÀ age
	policy := mustCompile(t, "P-FAILCLOSED", `permit(principal == any, action == any, resource == any)
when {
    context.ip_address in "192.168.0.0/16" &&
    context.age >= 18
};`)

	eng := newEngineWith(t, tenantID, []*parser.PolicyNode{policy}, nil)

	tests := []struct {
		name    string
		context map[string]string
	}{
		{
			name:    "Thiếu hoàn toàn context → DENY",
			context: map[string]string{},
		},
		{
			name: "Có ip_address nhưng thiếu age → DENY",
			context: map[string]string{
				"ip_address": "192.168.1.50",
			},
		},
		{
			name: "Có age nhưng thiếu ip_address → DENY",
			context: map[string]string{
				"age": "25",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Không được panic
			res := eng.CheckPermission(context.Background(), tenantID, "user:alice", "READ", "file:doc", tt.context)
			if res.Decision != DecisionDeny {
				t.Errorf("Fail-closed violation: mong đợi DENY khi thiếu thuộc tính, thực tế %v", res.Decision)
			}
		})
	}
}

// --- TestEvaluator_ForbidOverridesPermit ---

// Quy tắc "forbid overrides permit": dù có luật permit khớp,
// nếu có bất kỳ forbid nào khớp thì kết quả luôn là DENY.
func TestEvaluator_ForbidOverridesPermit(t *testing.T) {
	const tenantID = "tenant-forbid-test"

	permitPolicy := mustCompile(t, "P-PERMIT", `permit(principal == any, action == any, resource == any)
when {
    context.ip_address in "192.168.0.0/16"
};`)

	forbidPolicy := mustCompile(t, "P-FORBID", `forbid(principal == any, action == any, resource == any)
when {
    context.device_status == "compromised"
};`)

	eng := newEngineWith(t, tenantID, []*parser.PolicyNode{permitPolicy, forbidPolicy}, nil)

	tests := []struct {
		name     string
		context  map[string]string
		expected Decision
	}{
		{
			name: "Permit khớp, forbid không khớp → ALLOW",
			context: map[string]string{
				"ip_address":    "192.168.1.1",
				"device_status": "secure",
			},
			expected: DecisionAllow,
		},
		{
			name: "Permit khớp VÀ forbid khớp → DENY (forbid thắng)",
			context: map[string]string{
				"ip_address":    "192.168.1.1",
				"device_status": "compromised",
			},
			expected: DecisionDeny,
		},
		{
			name: "Chỉ có forbid khớp (permit không khớp do IP sai) → DENY",
			context: map[string]string{
				"ip_address":    "10.0.0.1",
				"device_status": "compromised",
			},
			expected: DecisionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := eng.CheckPermission(context.Background(), tenantID, "user:alice", "READ", "file:doc", tt.context)
			if res.Decision != tt.expected {
				t.Errorf("%s: mong đợi %v, thực tế %v (lý do: %s)", tt.name, tt.expected, res.Decision, res.Reason)
			}
		})
	}
}

// --- TestEvaluator_DenyByDefault ---

// Nguyên tắc deny-by-default: không có policy nào match → phải là DENY.
func TestEvaluator_DenyByDefault(t *testing.T) {
	const tenantID = "tenant-dbd-test"

	// Policy chỉ cho phép user:admin
	policy := mustCompile(t, "P-ADMIN-ONLY", `permit(principal == user:"admin", action == any, resource == any)
when {
    context.ip_address in "10.0.0.0/8"
};`)

	eng := newEngineWith(t, tenantID, []*parser.PolicyNode{policy}, nil)

	// user:nobody không khớp policy nào → DENY
	res := eng.CheckPermission(context.Background(), tenantID, "user:nobody", "READ", "file:secret",
		map[string]string{"ip_address": "10.1.1.1"})

	if res.Decision != DecisionDeny {
		t.Errorf("Deny-by-default violation: user không có policy match phải bị DENY, thực tế %v", res.Decision)
	}
}

// --- TestEvaluator_PoolReuse ---

// Kiểm tra EvalContext pool không để rò rỉ context giữa các request.
// Sau khi Release, dữ liệu cũ không được xuất hiện ở request tiếp theo.
func TestEvaluator_PoolReuse(t *testing.T) {
	const tenantID = "tenant-pool-test"

	// Chỉ cho phép khi device_status == "secure"
	policy := mustCompile(t, "P-POOL", `permit(principal == any, action == any, resource == any)
when {
    context.device_status == "secure"
};`)

	eng := newEngineWith(t, tenantID, []*parser.PolicyNode{policy}, nil)

	// Request 1: có device_status = "secure" → ALLOW
	res1 := eng.CheckPermission(context.Background(), tenantID, "user:alice", "READ", "file:doc",
		map[string]string{"device_status": "secure"})
	if res1.Decision != DecisionAllow {
		t.Fatalf("Request 1: mong đợi ALLOW, thực tế %v", res1.Decision)
	}

	// Request 2: context rỗng → DENY (context từ request 1 không được rò rỉ)
	res2 := eng.CheckPermission(context.Background(), tenantID, "user:bob", "READ", "file:doc",
		map[string]string{})
	if res2.Decision != DecisionDeny {
		t.Errorf("Pool leak: context rỗng phải là DENY. Có thể context pool bị rò rỉ từ request trước. Thực tế: %v", res2.Decision)
	}
}
