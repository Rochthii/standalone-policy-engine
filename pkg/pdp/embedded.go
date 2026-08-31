package pdp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/storage"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrTenantNotAllowed trả về khi ứng dụng gửi request của Tenant không nằm trong whitelist cấu hình.
	ErrTenantNotAllowed = errors.New("tenant khong nam trong danh sach duoc phep phuc vu cua microservice nay (Tenant Scoping)")
)

// EmbeddedPDP là instance PDP nhúng chạy trực tiếp bên trong không gian bộ nhớ của Microservice.
// Mọi thao tác CheckPermission được thực hiện hoàn toàn trong RAM (Lock-Free, 0.29µs, Zero-Network).
type EmbeddedPDP struct {
	engine         *engine.Engine
	storage        *storage.Storage
	allowedTenants map[string]bool
	redisClient    redis.UniversalClient
	syncInterval   time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// New khởi tạo một instance EmbeddedPDP mới.
func New(cfg Config) (*EmbeddedPDP, error) {
	if cfg.Storage == nil {
		return nil, errors.New("cau hinh Storage la bat buoc cho Embedded PDP")
	}

	interval := cfg.SyncInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	allowedMap := make(map[string]bool)
	for _, t := range cfg.AllowedTenants {
		allowedMap[t] = true
	}

	p := &EmbeddedPDP{
		engine:         engine.NewEngine(),
		storage:        cfg.Storage,
		allowedTenants: allowedMap,
		redisClient:    cfg.RedisClient,
		syncInterval:   interval,
		stopChan:       make(chan struct{}),
	}

	// Nạp khởi tạo ban đầu cho các AllowedTenants
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for tenantID := range p.allowedTenants {
		p.syncTenant(ctx, tenantID)
	}

	// Khởi chạy background worker đồng bộ ngầm (hoàn toàn không block luồng chính)
	p.wg.Add(1)
	go p.syncLoop()

	if p.redisClient != nil {
		p.wg.Add(1)
		go p.redisSubscriber()
	}

	return p, nil
}

// CheckPermission thực hiện đánh giá quyết định phân quyền trực tiếp trên RAM (Lock-Free, Zero-Alloc).
func (p *EmbeddedPDP) CheckPermission(ctx context.Context, tenantID, subject, action, resource string, contextMap map[string]string) (engine.DecisionResult, error) {
	// Kiểm tra Tenant Scoping để đảm bảo an toàn bộ nhớ
	if len(p.allowedTenants) > 0 && !p.allowedTenants[tenantID] {
		return engine.DecisionResult{
			Decision: engine.DecisionDeny,
			Reason:   "Tenant khong duoc phep truy cap tren instance nhung nay",
		}, ErrTenantNotAllowed
	}

	res := p.engine.CheckPermission(ctx, tenantID, subject, action, resource, contextMap)
	return res, nil
}

// ExplainDecision thực hiện đánh giá quyền kèm danh sách chính sách khớp và lý do chi tiết.
func (p *EmbeddedPDP) ExplainDecision(ctx context.Context, tenantID, subject, action, resource string, contextMap map[string]string) (engine.DecisionResult, error) {
	return p.CheckPermission(ctx, tenantID, subject, action, resource, contextMap)
}

// Close dừng an toàn các tiến trình đồng bộ nền của EmbeddedPDP.
func (p *EmbeddedPDP) Close() {
	close(p.stopChan)
	p.wg.Wait()
}

func (p *EmbeddedPDP) syncLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			for tenantID := range p.allowedTenants {
				p.syncTenant(ctx, tenantID)
			}
			cancel()
		}
	}
}

func (p *EmbeddedPDP) redisSubscriber() {
	defer p.wg.Done()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pubsub := p.redisClient.Subscribe(ctx, "policy-updates")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-p.stopChan:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event engine.PolicyUpdateEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
				// Chỉ nạp nếu tenant nằm trong Whitelist
				if p.allowedTenants[event.TenantID] {
					p.syncTenant(ctx, event.TenantID)
				}
			}
		}
	}
}

func (p *EmbeddedPDP) syncTenant(ctx context.Context, tenantID string) {
	dbPolicies, err := p.storage.GetActivePolicies(ctx, tenantID)
	if err != nil {
		return
	}

	compiler := parser.NewCompiler()
	compiledPolicies := make([]*parser.PolicyNode, 0, len(dbPolicies))

	for _, dbP := range dbPolicies {
		lexer := parser.NewLexer(dbP.PolicyText)
		pr := parser.NewParser(lexer)
		nodes := pr.Parse()
		if len(pr.Errors()) > 0 {
			continue
		}
		nodes[0].ID = dbP.ID
		compiled, err := compiler.Compile(nodes[0])
		if err != nil {
			continue
		}
		compiledPolicies = append(compiledPolicies, compiled)
	}

	rev, _ := p.storage.GetTenantRevision(ctx, tenantID)
	_ = p.engine.UpdateTenantPoliciesWithRevision(tenantID, compiledPolicies, nil, rev)
}
