package engine

import (
	"context"
	"encoding/json"
	"log"
	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/storage"
	"time"

	"github.com/redis/go-redis/v9"
)

// PolicyUpdateEvent mô tả định dạng sự kiện truyền nhận qua Redis Pub/Sub.
type PolicyUpdateEvent struct {
	TenantID string `json:"tenant_id"`
	PolicyID string `json:"policy_id"`
	Action   string `json:"action"` // UPDATE hoặc DELETE
}

// Syncer chịu trách nhiệm đồng bộ trạng thái chính sách giữa PostgreSQL (Source of Truth)
// và bộ nhớ RAM Trie Indexer của PDP Engine.
type Syncer struct {
	engine      *Engine
	storage     *storage.Storage
	redisClient *redis.Client
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewSyncer khởi tạo một instance Syncer.
func NewSyncer(eng *Engine, store *storage.Storage, rdb *redis.Client) *Syncer {
	return &Syncer{
		engine:      eng,
		storage:     store,
		redisClient: rdb,
		stopChan:    make(chan struct{}),
	}
}

// Start khởi chạy tiến trình đồng bộ (lắng nghe Redis và Polling dự phòng).
func (s *Syncer) Start(ctx context.Context) {
	s.wg.Add(2)

	// Worker 1: Lắng nghe kênh Redis Pub/Sub
	go s.redisSubscriber(ctx)

	// Worker 2: Polling định kỳ mỗi 10 giây để backup khi Redis mất kết nối
	go s.pollingWorker(ctx)
}

// Stop dừng an toàn Syncer.
func (s *Syncer) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *Syncer) redisSubscriber(ctx context.Context) {
	defer s.wg.Done()

	if s.redisClient == nil {
		log.Println("[Syncer] Redis Client không được cấu hình. Bỏ qua chế độ Pub/Sub.")
		return
	}

	pubsub := s.redisClient.Subscribe(ctx, "policy-updates")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("[Syncer] Bắt đầu lắng nghe kênh đồng bộ Redis 'policy-updates'...")

	for {
		select {
		case <-s.stopChan:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var event PolicyUpdateEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("[Syncer] Lỗi phân tích thông điệp cập nhật: %v", err)
				continue
			}

			log.Printf("[Syncer] Nhận sự kiện cập nhật nóng cho Tenant: %s (Action: %s)", event.TenantID, event.Action)
			s.SyncTenant(ctx, event.TenantID)
		}
	}
}

func (s *Syncer) pollingWorker(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			// Trong kịch bản thực tế, ta chỉ cần quét các Tenant hoạt động gần đây
			// để tránh lãng phí truy vấn DB. Ở đây, ta lấy tất cả các tenant hiện có trên Engine.
			state := s.engine.GetState()
			for tenantID := range state.Tenants {
				// Gọi đồng bộ
				s.SyncTenant(ctx, tenantID)
			}
		}
	}
}

// SyncTenant thực hiện nạp lại toàn bộ chính sách ACTIVE từ PostgreSQL cho một Tenant,
// biên dịch AST và hoán đổi Trie Index RAM.
func (s *Syncer) SyncTenant(ctx context.Context, tenantID string) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 1. Lấy danh sách chính sách ACTIVE của Tenant từ DB
	dbPolicies, err := s.storage.GetActivePolicies(dbCtx, tenantID)
	if err != nil {
		log.Printf("[Syncer] Lỗi truy xuất chính sách từ PostgreSQL cho Tenant %s: %v", tenantID, err)
		return
	}

	// 2. Biên dịch (Compile) các chính sách sang cây AST
	compiledPolicies := make([]*parser.PolicyNode, 0, len(dbPolicies))
	compiler := parser.NewCompiler()

	for _, dbP := range dbPolicies {
		lexer := parser.NewLexer(dbP.PolicyText)
		pr := parser.NewParser(lexer)
		nodes := pr.Parse()
		if len(pr.Errors()) > 0 {
			log.Printf("[Syncer] Lỗi parse chính sách %s: %v. Bỏ qua chính sách này.", dbP.ID, pr.Errors())
			continue
		}

		nodes[0].ID = dbP.ID
		compiled, err := compiler.Compile(nodes[0])
		if err != nil {
			log.Printf("[Syncer] Lỗi compile chính sách %s: %v. Bỏ qua.", dbP.ID, err)
			continue
		}

		compiledPolicies = append(compiledPolicies, compiled)
	}

	// 3. Cập nhật nóng vào RAM Trie thông qua Copy-On-Write (COW)
	// (Ở giai đoạn này ta không cập nhật thừa kế vai trò, nhưng có thể mở rộng sau)
	err = s.engine.UpdateTenantPolicies(tenantID, compiledPolicies, nil)
	if err != nil {
		log.Printf("[Syncer] Lỗi cập nhật RAM Trie cho Tenant %s: %v", tenantID, err)
	} else {
		metrics.UpdateActivePoliciesCount(tenantID, len(compiledPolicies))
		log.Printf("[Syncer] Đồng bộ thành công %d chính sách lên RAM cho Tenant %s", len(compiledPolicies), tenantID)
	}
}

// import sync for WaitGroup
import "sync"
