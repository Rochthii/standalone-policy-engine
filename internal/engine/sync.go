package engine

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/storage"
)

// PolicyUpdateEvent mô tả định dạng sự kiện đồng bộ qua PostgreSQL LISTEN/NOTIFY.
type PolicyUpdateEvent struct {
	TenantID string `json:"tenant_id"`
	PolicyID string `json:"policy_id"`
	Action   string `json:"action"`   // UPDATE hoặc DELETE
	Revision uint64 `json:"revision"` // Monotonic Revision ID tăng dần đơn điệu
}

// SyncStatus định nghĩa trạng thái sức khỏe của luồng đồng bộ phân tán.
type SyncStatus string

const (
	SyncStatusHealthy  SyncStatus = "HEALTHY"
	SyncStatusDegraded SyncStatus = "DEGRADED"
)

// Syncer chịu trách nhiệm đồng bộ trạng thái chính sách giữa PostgreSQL (Source of Truth)
// và bộ nhớ RAM Trie Indexer của PDP Engine qua PostgreSQL LISTEN/NOTIFY và Gap Recovery.
type Syncer struct {
	engine      *EngineWithGC
	storage     *storage.Storage
	badgerStore *storage.BadgerStore
	statusMu    sync.RWMutex
	status      SyncStatus
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewSyncer khởi tạo một instance Syncer không phụ thuộc Redis.
func NewSyncer(eng *EngineWithGC, store *storage.Storage) *Syncer {
	return &Syncer{
		engine:   eng,
		storage:  store,
		status:   SyncStatusHealthy,
		stopChan: make(chan struct{}),
	}
}

// SetBadgerStore thiết lập tầng lưu trữ cục bộ BadgerDB (dành riêng cho Edge Mode).
func (s *Syncer) SetBadgerStore(badger *storage.BadgerStore) {
	s.badgerStore = badger
}

// GetSyncStatus trả về trạng thái đồng bộ phân tán hiện tại (HEALTHY hoặc DEGRADED).
func (s *Syncer) GetSyncStatus() SyncStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

func (s *Syncer) setSyncStatus(st SyncStatus) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status = st
}

// Start khởi chạy tiến trình lắng nghe sự kiện từ PostgreSQL.
func (s *Syncer) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.postgresEventSubscriber(ctx)
}

// Stop dừng an toàn Syncer.
func (s *Syncer) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *Syncer) postgresEventSubscriber(ctx context.Context) {
	defer s.wg.Done()

	log.Println("[Syncer] Khởi chạy worker lắng nghe PostgreSQL LISTEN 'policy_events'...")

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		default:
		}

		s.setSyncStatus(SyncStatusHealthy)
		err := s.storage.ListenPolicyEvents(ctx, func(ev storage.DBPolicyUpdateEvent) {
			currentRev := s.engine.GetTenantRevision(ev.TenantID)
			if ev.Revision > 0 && ev.Revision > currentRev+1 {
				log.Printf("[Syncer] Gap Detected: current=%d, received=%d cho Tenant %s. Kích hoạt Fast Catch-Up Sync ngay lập tức (<50ms)!", currentRev, ev.Revision, ev.TenantID)
			} else {
				log.Printf("[Syncer] Nhận thông điệp đồng bộ cho Tenant: %s (Action: %s, Revision: %d)", ev.TenantID, ev.Action, ev.Revision)
			}
			s.SyncTenantWithRevision(ctx, ev.TenantID, ev.Revision)
		})

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setSyncStatus(SyncStatusDegraded)
			log.Printf("[Syncer] Mất kết nối LISTEN PostgreSQL (%v). Tự động kết nối lại và đối soát sau 1 giây...", err)

			select {
			case <-s.stopChan:
				return
			case <-time.After(1 * time.Second):
				state := s.engine.GetState()
				for tenantID := range state.Tenants {
					s.reconcileTenantRevision(ctx, tenantID)
				}
			}
		}
	}
}

func (s *Syncer) reconcileTenantRevision(ctx context.Context, tenantID string) {
	dbRev, err := s.storage.GetTenantRevision(ctx, tenantID)
	if err != nil {
		return
	}
	currentRev := s.engine.GetTenantRevision(tenantID)
	if dbRev > currentRev {
		log.Printf("[Syncer] Đối soát sau kết nối lại: Tenant %s DB revision %d > RAM revision %d. Đồng bộ bù tức thì!", tenantID, dbRev, currentRev)
		s.SyncTenantWithRevision(ctx, tenantID, dbRev)
	}
}

// SyncTenant thực hiện nạp lại toàn bộ chính sách ACTIVE từ PostgreSQL cho một Tenant.
func (s *Syncer) SyncTenant(ctx context.Context, tenantID string) {
	s.SyncTenantWithRevision(ctx, tenantID, 0)
}

// SyncTenantWithRevision thực hiện nạp lại chính sách từ DB và cập nhật với Revision ID cụ thể.
func (s *Syncer) SyncTenantWithRevision(ctx context.Context, tenantID string, revision uint64) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	dbPolicies, err := s.storage.GetActivePolicies(dbCtx, tenantID)
	if err != nil {
		log.Printf("[Syncer] Lỗi truy xuất chính sách từ PostgreSQL cho Tenant %s: %v", tenantID, err)
		return
	}

	compiledPolicies := make([]*parser.PolicyNode, 0, len(dbPolicies))
	compiler := parser.NewCompiler()

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

	if revision == 0 {
		revision = s.engine.GetTenantRevision(tenantID) + 1
	}

	err = s.engine.UpdateTenantPoliciesWithRevision(tenantID, compiledPolicies, nil, revision)
	if err != nil {
		log.Printf("[Syncer] Lỗi cập nhật RAM Trie cho Tenant %s: %v", tenantID, err)
	} else {
		metrics.UpdateActivePoliciesCount(tenantID, len(compiledPolicies))
		log.Printf("[Syncer] Đồng bộ thành công %d chính sách (Revision: %d) lên RAM cho Tenant %s", len(compiledPolicies), revision, tenantID)

		if s.badgerStore != nil {
			rawList := make([]json.RawMessage, 0, len(dbPolicies))
			for _, dbP := range dbPolicies {
				if len(dbP.ASTJSON) > 0 {
					rawList = append(rawList, dbP.ASTJSON)
				}
			}
			snapshot := &storage.PolicySnapshot{
				TenantID:   tenantID,
				Policies:   rawList,
				SnapshotAt: time.Now(),
			}
			_ = s.badgerStore.SavePolicySnapshot(snapshot)
		}
	}
}
