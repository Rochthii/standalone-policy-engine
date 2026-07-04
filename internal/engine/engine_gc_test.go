package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"standalone-policy-engine/internal/parser"
)

// mockLoader theo dõi các lần Lazy Loading được gọi và nạp chính sách giả lập.
type mockLoader struct {
	mu          sync.Mutex
	loadedCount int
	policies    []*parser.PolicyNode
	eng         *EngineWithGC
}

func (m *mockLoader) load(ctx context.Context, tenantID string) error {
	m.mu.Lock()
	m.loadedCount++
	m.mu.Unlock()
	// Giả lập nạp chính sách vào Engine
	return m.eng.UpdateTenantPolicies(tenantID, m.policies, nil)
}

// compileTestPolicy biên dịch nhanh một chính sách DSL từ string.
func compileTestPolicy(t *testing.T, id, dsl string) *parser.PolicyNode {
	t.Helper()
	lexer := parser.NewLexer(dsl)
	pr := parser.NewParser(lexer)
	nodes := pr.Parse()
	if len(pr.Errors()) > 0 {
		t.Fatalf("Lỗi parse chính sách [%s]: %v", id, pr.Errors())
	}
	nodes[0].ID = id
	c := parser.NewCompiler()
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("Lỗi compile chính sách [%s]: %v", id, err)
	}
	return compiled
}

func TestGCUnloadIdleTenant(t *testing.T) {
	// GC interval 100ms, maxIdleTime 50ms để test nhanh
	eng := NewEngineWithGC(100*time.Millisecond, 50*time.Millisecond)

	tenantID := "tenant-gc-test"
	dsl := `permit(principal == user:alice, action == action:READ, resource == file:doc.txt) when { true };`
	policies := []*parser.PolicyNode{compileTestPolicy(t, "P-GC-001", dsl)}

	if err := eng.UpdateTenantPolicies(tenantID, policies, nil); err != nil {
		t.Fatalf("UpdateTenantPolicies lỗi: %v", err)
	}

	// Xác nhận Tenant đã nạp
	state := eng.GetState()
	if _, exists := state.Tenants[tenantID]; !exists {
		t.Fatal("Tenant phải tồn tại sau khi nạp chính sách")
	}

	// Chờ maxIdleTime trôi qua (không gọi CheckPermission)
	time.Sleep(200 * time.Millisecond)

	// Chạy GC cycle thủ công
	eng.runGCCycle()

	// Xác nhận Tenant đã bị unload
	state = eng.GetState()
	if _, exists := state.Tenants[tenantID]; exists {
		t.Error("Tenant phải đã bị GC unload sau khi hết maxIdleTime")
	}
}

func TestGCLazyLoadingOnRequest(t *testing.T) {
	eng := NewEngineWithGC(1*time.Hour, 1*time.Hour) // GC sẽ không chạy tự động

	tenantID := "tenant-lazy-test"
	dsl := `permit(principal == user:alice, action == action:READ, resource == file:doc.txt) when { true };`
	policies := []*parser.PolicyNode{compileTestPolicy(t, "P-LZ-001", dsl)}

	loader := &mockLoader{policies: policies, eng: eng}
	eng.SetLazyLoader(loader.load)

	// Tenant CHƯA được nạp → GetTenantTrie sẽ kích hoạt lazy load
	ctx := context.Background()
	trie, exists := eng.GetTenantTrie(ctx, tenantID)

	if !exists {
		t.Error("Lazy Loading phải đã nạp Tenant và trả về Trie")
	}
	if trie == nil {
		t.Error("Trie phải khác nil sau Lazy Loading")
	}

	loader.mu.Lock()
	count := loader.loadedCount
	loader.mu.Unlock()
	if count != 1 {
		t.Errorf("LazyLoader phải được gọi đúng 1 lần, thực tế: %d", count)
	}

	// Lần sau truy cập lại cùng Tenant → Lazy Loading không được gọi thêm
	eng.GetTenantTrie(ctx, tenantID)
	loader.mu.Lock()
	count = loader.loadedCount
	loader.mu.Unlock()
	if count != 1 {
		t.Errorf("LazyLoader phải gọi đúng 1 lần (cache hit), thực tế: %d", count)
	}
}

func TestGCTenantStaysAliveOnActivity(t *testing.T) {
	// maxIdleTime 100ms nhưng ta sẽ đụng vào mỗi 40ms → Tenant phải tồn tại
	eng := NewEngineWithGC(1*time.Hour, 100*time.Millisecond)

	tenantID := "tenant-alive"
	dsl := `permit(principal == user:bob, action == action:WRITE, resource == file:test.txt) when { true };`
	policies := []*parser.PolicyNode{compileTestPolicy(t, "P-ALIVE-001", dsl)}

	if err := eng.UpdateTenantPolicies(tenantID, policies, nil); err != nil {
		t.Fatalf("UpdateTenantPolicies lỗi: %v", err)
	}

	// Giữ Tenant alive bằng cách touch mỗi 40ms trong 200ms
	for i := 0; i < 5; i++ {
		time.Sleep(40 * time.Millisecond)
		eng.CheckPermission(tenantID, "user:bob", "WRITE", "file:test.txt", nil)
	}

	// Chạy GC cycle
	eng.runGCCycle()

	// Tenant phải vẫn còn sống vì vừa được touch
	state := eng.GetState()
	if _, exists := state.Tenants[tenantID]; !exists {
		t.Error("Tenant phải còn sống vì liên tục có activity")
	}
}

func TestGCConcurrentSafety(t *testing.T) {
	eng := NewEngineWithGC(50*time.Millisecond, 30*time.Millisecond)

	tenantIDs := []string{"t1", "t2", "t3", "t4", "t5"}
	dsl := `permit(principal == user:x, action == action:READ, resource == file:x.txt) when { true };`

	for _, tid := range tenantIDs {
		policies := []*parser.PolicyNode{compileTestPolicy(t, "P-CONC-"+tid, dsl)}
		_ = eng.UpdateTenantPolicies(tid, policies, nil)
	}

	ctx, cancel := context.WithCancel(context.Background())
	eng.StartGC(ctx)

	var wg sync.WaitGroup
	// Chạy 20 goroutine đọc đồng thời trong khi GC đang chạy
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				for _, tid := range tenantIDs {
					eng.CheckPermission(tid, "user:x", "READ", "file:x.txt", nil)
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
	cancel()
	// Không có race condition → test pass
}
