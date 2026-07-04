package engine

import (
	"fmt"
	"standalone-policy-engine/internal/parser"
	"sync"
	"testing"
	"time"
)

func TestRoleDAG(t *testing.T) {
	dag := NewRoleDAG()

	// super_admin -> admin -> operator
	if err := dag.AddInheritance("super_admin", "admin"); err != nil {
		t.Fatalf("Lỗi thêm kế thừa: %v", err)
	}
	if err := dag.AddInheritance("admin", "operator"); err != nil {
		t.Fatalf("Lỗi thêm kế thừa: %v", err)
	}

	// Kiểm tra kế thừa
	if !dag.IsDescendant("super_admin", "operator") {
		t.Error("Mong đợi super_admin kế thừa operator")
	}
	if !dag.IsDescendant("super_admin", "admin") {
		t.Error("Mong đợi super_admin kế thừa admin")
	}
	if dag.IsDescendant("operator", "super_admin") {
		t.Error("Không mong đợi operator kế thừa super_admin")
	}

	// Thử thêm quan hệ vòng lặp: operator kế thừa super_admin (bị cấm)
	err := dag.AddInheritance("operator", "super_admin")
	if err == nil {
		t.Fatal("Mong đợi lỗi phát hiện chu trình (cycle detection), nhưng không xảy ra lỗi")
	}
	if !strings.Contains(err.Error(), "phát hiện quan hệ thừa kế vòng lặp") {
		t.Errorf("Thông điệp lỗi sai: %v", err)
	}
}

func TestEngine_DecisionFlow(t *testing.T) {
	engine := NewEngine()

	// Biên dịch các chính sách mẫu bằng Lexer/Parser/Compiler
	c := parser.NewCompiler()

	// Policy 1: Cho phép nhân viên kế thừa vai trò 'operator' đọc tài liệu khi kết nối từ IP văn phòng và đủ 18 tuổi
	p1Str := `permit(principal in role:operator, action == action:READ, resource == any)
when {
	context.ip_address in "192.168.1.0/24" &&
	context.age >= 18
};`
	p1Node := compileHelper(t, c, "P-001", p1Str)

	// Policy 2: Cấm tường minh bất kỳ ai truy cập nếu thiết bị không an toàn (forbid overrides)
	p2Str := `forbid(principal == any, action == any, resource == any)
when {
	context.device_status == "compromised"
};`
	p2Node := compileHelper(t, c, "P-002", p2Str)

	// Nạp tập luật vào Engine cho Tenant "tenant-1"
	policies := []*parser.PolicyNode{p1Node, p2Node}
	// Phân cấp vai trò: user:alice thuộc vai trò role:operator
	inheritances := [][2]string{
		{"user:alice", "role:operator"},
	}

	err := engine.UpdateTenantPolicies("tenant-1", policies, inheritances)
	if err != nil {
		t.Fatalf("Lỗi nạp chính sách: %v", err)
	}

	tests := []struct {
		name             string
		subject          string
		action           string
		resource         string
		context          map[string]string
		expectedDecision Decision
		expectedReason   string
	}{
		{
			name:     "Thỏa mãn điều kiện permit -> ALLOW",
			subject:  "user:alice",
			action:   "READ",
			resource: "file:report.pdf",
			context: map[string]string{
				"ip_address": "192.168.1.50",
				"age":        "20",
			},
			expectedDecision: DecisionAllow,
			expectedReason:   "P-001",
		},
		{
			name:     "Ip address không thỏa mãn -> DENY (Default)",
			subject:  "user:alice",
			action:   "READ",
			resource: "file:report.pdf",
			context: map[string]string{
				"ip_address": "10.0.0.1",
				"age":        "20",
			},
			expectedDecision: DecisionDeny,
			expectedReason:   "Không tìm thấy luật cho phép",
		},
		{
			name:     "Thiếu thuộc tính age (safe fail-closed) -> DENY (Default)",
			subject:  "user:alice",
			action:   "READ",
			resource: "file:report.pdf",
			context: map[string]string{
				"ip_address": "192.168.1.50",
			},
			expectedDecision: DecisionDeny,
			expectedReason:   "Không tìm thấy luật cho phép",
		},
		{
			name:     "Thỏa mãn permit nhưng dính forbid (device status compromised) -> DENY (Forbid Override)",
			subject:  "user:alice",
			action:   "READ",
			resource: "file:report.pdf",
			context: map[string]string{
				"ip_address":    "192.168.1.50",
				"age":           "20",
				"device_status": "compromised",
			},
			expectedDecision: DecisionDeny,
			expectedReason:   "P-002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := engine.CheckPermission("tenant-1", tt.subject, tt.action, tt.resource, tt.context)
			if res.Decision != tt.expectedDecision {
				t.Errorf("Quyết định sai. mong đợi=%v, thực tế=%v, lý do=%s", tt.expectedDecision, res.Decision, res.Reason)
			}
			if tt.expectedDecision == DecisionAllow {
				if len(res.Explanations) == 0 || res.Explanations[0] != tt.expectedReason {
					t.Errorf("Giải thích sai. mong đợi chứa=%s, thực tế=%v", tt.expectedReason, res.Explanations)
				}
			} else {
				if strings.Contains(tt.expectedReason, "P-") {
					found := false
					for _, exp := range res.Explanations {
						if exp == tt.expectedReason {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Giải thích sai. mong đợi chứa=%s, thực tế=%v", tt.expectedReason, res.Explanations)
					}
				}
			}
		})
	}
}

func TestEngine_ConcurrencyCOW(t *testing.T) {
	engine := NewEngine()
	c := parser.NewCompiler()

	pStr := `permit(principal == any, action == any, resource == any)
when { context.ip_address in "192.168.1.0/24" };`
	pNode := compileHelper(t, c, "P-CONC", pStr)

	// Khởi tạo tập luật ban đầu
	err := engine.UpdateTenantPolicies("tenant-conc", []*parser.PolicyNode{pNode}, nil)
	if err != nil {
		t.Fatalf("Lỗi khởi tạo Engine: %v", err)
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// 1. Chạy 20 Goroutine đọc gRPC CheckPermission liên tục (Read-Heavy)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopChan:
					return
				default:
					res := engine.CheckPermission("tenant-conc", "user:random", "READ", "file:doc", map[string]string{
						"ip_address": "192.168.1.100",
					})
					if res.Decision != DecisionAllow {
						t.Errorf("Đọc luồng %d: Kỳ vọng ALLOW, thực tế: %v, lý do: %s", id, res.Decision, res.Reason)
					}
					// Sleep siêu ngắn để nhường thread
					time.Sleep(10 * time.Microsecond)
				}
			}
		}(i)
	}

	// 2. Chạy 3 Goroutine ghi (Admin Hot-Reload) cập nhật chính sách liên tục (COW)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(2 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					// Cập nhật nóng
					err := engine.UpdateTenantPolicies("tenant-conc", []*parser.PolicyNode{pNode}, nil)
					if err != nil {
						t.Errorf("Ghi luồng %d: Cập nhật thất bại: %v", id, err)
					}
				}
			}
		}(i)
	}

	// Cho test chạy trong 300ms
	time.Sleep(300 * time.Millisecond)
	close(stopChan)
	wg.Wait()
}

func compileHelper(t *testing.T, c *parser.Compiler, id string, dsl string) *parser.PolicyNode {
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

// import strings for tests
import "strings"
