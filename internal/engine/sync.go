package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	SyncStatusHealthy  SyncStatus = "healthy"
	SyncStatusDegraded SyncStatus = "degraded"
	SyncStatusNotReady SyncStatus = "not_ready"
)

// Syncer chịu trách nhiệm đồng bộ trạng thái chính sách giữa PostgreSQL (Source of Truth)
// và bộ nhớ RAM Trie Indexer của PDP Engine qua PostgreSQL LISTEN/NOTIFY và Gap Recovery.
type Syncer struct {
	engine            *EngineWithGC
	storage           policySyncStorage
	badgerStore       *storage.BadgerStore
	reconcileInterval time.Duration
	statusMu          sync.RWMutex
	status            SyncStatus
	lifecycleMu       sync.Mutex
	cancel            context.CancelFunc
	startOnce         sync.Once
	stopOnce          sync.Once
	wg                sync.WaitGroup
}

type policySyncStorage interface {
	ListenPolicyEvents(context.Context, func(storage.DBPolicyUpdateEvent), func()) error
	GetTenantRevision(context.Context, string) (uint64, error)
	GetTenantPolicyBundle(context.Context, string) (*storage.TenantPolicyBundle, error)
}

// NewSyncer khởi tạo một instance Syncer không phụ thuộc Redis.
func NewSyncer(eng *EngineWithGC, store policySyncStorage, reconcileInterval time.Duration) *Syncer {
	if reconcileInterval <= 0 {
		reconcileInterval = 10 * time.Second
	}
	return &Syncer{
		engine:            eng,
		storage:           store,
		reconcileInterval: reconcileInterval,
		status:            SyncStatusNotReady,
	}
}

// Readiness verifies that the policy listener is healthy and that every loaded
// tenant has the same policy revision in memory and PostgreSQL.
func (s *Syncer) Readiness(ctx context.Context) (SyncStatus, error) {
	status := s.GetSyncStatus()
	if status != SyncStatusHealthy {
		return status, fmt.Errorf("policy sync is %s", status)
	}
	for tenantID, trie := range s.engine.GetState().Tenants {
		revision, err := s.storage.GetTenantRevision(ctx, tenantID)
		if err != nil {
			return SyncStatusDegraded, fmt.Errorf("read policy revision for tenant %s: %w", tenantID, err)
		}
		if revision != trie.Revision {
			return SyncStatusDegraded, fmt.Errorf("policy revision lag for tenant %s: memory=%d database=%d", tenantID, trie.Revision, revision)
		}
	}
	return SyncStatusHealthy, nil
}

// SetBadgerStore thiết lập tầng lưu trữ cục bộ BadgerDB (dành riêng cho Edge Mode).
func (s *Syncer) SetBadgerStore(badger *storage.BadgerStore) {
	s.badgerStore = badger
}

// GetSyncStatus returns the current distributed policy-sync health state.
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
	s.startOnce.Do(func() {
		workerCtx, cancel := context.WithCancel(ctx)
		s.lifecycleMu.Lock()
		s.cancel = cancel
		s.lifecycleMu.Unlock()
		s.wg.Add(2)
		go s.postgresEventSubscriber(workerCtx)
		go s.reconciliationWorker(workerCtx)
	})
}

// Stop dừng an toàn Syncer.
func (s *Syncer) Stop() {
	s.stopOnce.Do(func() {
		s.lifecycleMu.Lock()
		cancel := s.cancel
		s.lifecycleMu.Unlock()
		if cancel != nil {
			cancel()
		}
		s.wg.Wait()
	})
}

func (s *Syncer) postgresEventSubscriber(ctx context.Context) {
	defer s.wg.Done()

	log.Println("[Syncer] Khởi chạy worker lắng nghe PostgreSQL LISTEN 'policy_events'...")

	for {
		if ctx.Err() != nil {
			return
		}

		err := s.storage.ListenPolicyEvents(ctx, func(ev storage.DBPolicyUpdateEvent) {
			currentRev := s.engine.GetTenantRevision(ev.TenantID)
			if !shouldSyncRevision(currentRev, ev.Revision) {
				log.Printf("[Syncer] Bỏ qua event cũ/trùng cho Tenant %s: current=%d received=%d", ev.TenantID, currentRev, ev.Revision)
				return
			}
			if ev.Revision > 0 && ev.Revision > currentRev+1 {
				log.Printf("[Syncer] Gap Detected: current=%d, received=%d cho Tenant %s. Kích hoạt Fast Catch-Up Sync ngay lập tức (<50ms)!", currentRev, ev.Revision, ev.TenantID)
			} else {
				log.Printf("[Syncer] Nhận thông điệp đồng bộ cho Tenant: %s (Action: %s, Revision: %d)", ev.TenantID, ev.Action, ev.Revision)
			}
			if err := s.SyncTenantWithRevision(ctx, ev.TenantID, ev.Revision); err != nil {
				s.setSyncStatus(SyncStatusDegraded)
				log.Printf("[Syncer] Từ chối cập nhật Tenant %s; giữ last-known-good: %v", ev.TenantID, err)
			}
		}, func() {
			s.setSyncStatus(SyncStatusHealthy)
		})

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setSyncStatus(SyncStatusDegraded)
			log.Printf("[Syncer] Mất kết nối LISTEN PostgreSQL (%v). Tự động kết nối lại và đối soát sau 1 giây...", err)

			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (s *Syncer) reconciliationWorker(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileLoadedTenants(ctx)
		}
	}
}

func (s *Syncer) reconcileLoadedTenants(ctx context.Context) {
	state := s.engine.GetState()
	for tenantID := range state.Tenants {
		if err := s.reconcileTenantRevision(ctx, tenantID); err != nil {
			s.setSyncStatus(SyncStatusDegraded)
			log.Printf("[Syncer] Periodic reconcile failed for Tenant %s: %v", tenantID, err)
		}
	}
}

func (s *Syncer) reconcileTenantRevision(ctx context.Context, tenantID string) error {
	dbRev, err := s.storage.GetTenantRevision(ctx, tenantID)
	if err != nil {
		return err
	}
	currentRev := s.engine.GetTenantRevision(tenantID)
	if dbRev > currentRev {
		log.Printf("[Syncer] Đối soát sau kết nối lại: Tenant %s DB revision %d > RAM revision %d. Đồng bộ bù tức thì!", tenantID, dbRev, currentRev)
		if err := s.SyncTenantWithRevision(ctx, tenantID, dbRev); err != nil {
			return err
		}
	}
	return nil
}

// SyncTenant thực hiện nạp lại toàn bộ chính sách ACTIVE từ PostgreSQL cho một Tenant.
func (s *Syncer) SyncTenant(ctx context.Context, tenantID string) error {
	return s.SyncTenantWithRevision(ctx, tenantID, 0)
}

// SyncTenantWithRevision thực hiện nạp lại chính sách từ DB và cập nhật với Revision ID cụ thể.
func (s *Syncer) SyncTenantWithRevision(ctx context.Context, tenantID string, revision uint64) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bundle, err := s.storage.GetTenantPolicyBundle(dbCtx, tenantID)
	if err != nil {
		return fmt.Errorf("load policy bundle for tenant %s: %w", tenantID, err)
	}

	sources := make([]parser.PolicySource, 0, len(bundle.Policies))
	for _, dbP := range bundle.Policies {
		sources = append(sources, parser.PolicySource{ID: dbP.ID, Text: dbP.PolicyText})
	}
	compiledPolicies, err := parser.CompilePolicySet(sources)
	if err != nil {
		return fmt.Errorf("compile active policy set for tenant %s: %w", tenantID, err)
	}

	if revision > bundle.Revision {
		return fmt.Errorf("policy event revision %d is ahead of database bundle revision %d for tenant %s", revision, bundle.Revision, tenantID)
	}
	revision = bundle.Revision

	err = s.engine.UpdateTenantPoliciesWithRevision(tenantID, compiledPolicies, bundle.Inheritances, revision)
	if err != nil {
		if errors.Is(err, ErrStaleRevision) {
			return nil
		}
		return fmt.Errorf("publish in-memory policy set for tenant %s: %w", tenantID, err)
	}
	metrics.UpdateActivePoliciesCount(tenantID, len(compiledPolicies))
	log.Printf("[Syncer] Đồng bộ thành công %d chính sách (Revision: %d) lên RAM cho Tenant %s", len(compiledPolicies), revision, tenantID)

	if s.badgerStore != nil {
		rawList := make([]json.RawMessage, 0, len(bundle.Policies))
		for _, dbP := range bundle.Policies {
			if len(dbP.ASTJSON) > 0 {
				rawList = append(rawList, dbP.ASTJSON)
			}
		}
		snapshot := &storage.PolicySnapshot{
			TenantID:     tenantID,
			Policies:     rawList,
			Inheritances: bundle.Inheritances,
			SnapshotAt:   time.Now(),
		}
		if err := s.badgerStore.SavePolicySnapshot(snapshot); err != nil {
			return fmt.Errorf("save edge policy snapshot for tenant %s: %w", tenantID, err)
		}
	}
	return nil
}

func shouldSyncRevision(current, received uint64) bool {
	return received == 0 || received > current
}
