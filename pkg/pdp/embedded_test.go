package pdp

import (
	"context"
	"sync"
	"testing"
	"time"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/storage"
)

func TestEmbeddedPDP_TenantScoping(t *testing.T) {
	store, _ := storage.NewStorage("postgres://postgres:postgres@localhost:5432/policy_engine_test?sslmode=disable")
	if store == nil {
		// Mock config with dummy storage for unit test
		p := &EmbeddedPDP{
			engine:         engine.NewEngine(),
			allowedTenants: map[string]bool{"tenant-allowed": true},
			stopChan:       make(chan struct{}),
		}
		defer p.Close()

		// Nạp chính sách mẫu vào engine
		compiler := parser.NewCompiler()
		l := parser.NewLexer(`permit(principal == user:"alice", action == action:READ, resource == any);`)
		pr := parser.NewParser(l)
		nodes := pr.Parse()
		nodes[0].ID = "P-1"
		compiled, _ := compiler.Compile(nodes[0])
		_ = p.engine.UpdateTenantPolicies("tenant-allowed", []*parser.PolicyNode{compiled}, nil)

		ctx := context.Background()

		// 1. Kiểm tra Tenant nằm trong Whitelist -> Thành công
		res, err := p.CheckPermission(ctx, "tenant-allowed", "user:alice", "READ", "file:doc", nil)
		if err != nil {
			t.Fatalf("Không mong đợi lỗi cho allowed tenant: %v", err)
		}
		if res.Decision != engine.DecisionAllow {
			t.Errorf("Mong đợi ALLOW, thực tế: %v", res.Decision)
		}

		// 2. Kiểm tra Tenant KHÔNG nằm trong Whitelist -> Bị chặn ngay lập tức
		_, err = p.CheckPermission(ctx, "tenant-hacker", "user:alice", "READ", "file:doc", nil)
		if err != ErrTenantNotAllowed {
			t.Fatalf("Mong đợi lỗi ErrTenantNotAllowed, thực tế: %v", err)
		}
	}
}

func TestEmbeddedPDP_ConcurrentSafety(t *testing.T) {
	p := &EmbeddedPDP{
		engine:         engine.NewEngine(),
		allowedTenants: map[string]bool{"tenant-conc": true},
		stopChan:       make(chan struct{}),
	}
	defer p.Close()

	compiler := parser.NewCompiler()
	l := parser.NewLexer(`permit(principal == any, action == action:READ, resource == any);`)
	pr := parser.NewParser(l)
	nodes := pr.Parse()
	nodes[0].ID = "P-1"
	compiled, _ := compiler.Compile(nodes[0])
	_ = p.engine.UpdateTenantPolicies("tenant-conc", []*parser.PolicyNode{compiled}, nil)

	ctx := context.Background()
	var wg sync.WaitGroup
	stopTest := make(chan struct{})

	// 50 Goroutines đọc đồng thời
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopTest:
					return
				default:
					res, err := p.CheckPermission(ctx, "tenant-conc", "user:bob", "READ", "doc:1", nil)
					if err != nil || res.Decision != engine.DecisionAllow {
						t.Errorf("CheckPermission loi trong luong doc: %v, res: %v", err, res)
					}
				}
			}
		}()
	}

	// 3 Goroutines cập nhật state liên tục qua COW (giả lập background sync)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(2 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopTest:
					return
				case <-ticker.C:
					_ = p.engine.UpdateTenantPolicies("tenant-conc", []*parser.PolicyNode{compiled}, nil)
				}
			}
		}(i)
	}

	time.Sleep(200 * time.Millisecond)
	close(stopTest)
	wg.Wait()
}
