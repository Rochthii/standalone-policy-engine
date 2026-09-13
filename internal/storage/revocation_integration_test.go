package storage

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/testutil"
)

type delayedRevocationStore struct {
	store *Storage
	delay time.Duration
}

func (s *delayedRevocationStore) PersistRevocation(ctx context.Context, record security.RevocationRecord) (security.RevocationRecord, error) {
	return s.store.PersistRevocation(ctx, record)
}

func (s *delayedRevocationStore) WatchRevocations(ctx context.Context, snapshot func([]security.RevocationRecord), event func(security.RevocationRecord)) error {
	return s.store.WatchRevocations(ctx, snapshot, func(record security.RevocationRecord) {
		timer := time.NewTimer(s.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			event(record)
		}
	})
}

func TestRevocationPropagationThreeReplicasAndRestart(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL revocation integration")
	}
	ctx := context.Background()
	store, err := NewStorage(testutil.CreateIsolatedPostgresDatabase(t, ctx, adminURL))
	if err != nil {
		t.Fatalf("create isolated revocation store: %v", err)
	}
	t.Cleanup(store.Close)
	tenantID, err := store.CreateTenant(ctx, "revocation-three-replicas")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	managers := make([]*security.DelegationManager, 3)
	syncers := make([]*security.RevocationSyncer, 3)
	cancels := make([]context.CancelFunc, 3)
	watchStores := []security.RevocationStore{store, store, &delayedRevocationStore{store: store, delay: 25 * time.Millisecond}}
	for i := range managers {
		managers[i] = security.NewDelegationManagerWithSecret("integration-secret")
		watchCtx, cancel := context.WithCancel(ctx)
		cancels[i] = cancel
		syncers[i] = security.NewRevocationSyncer(managers[i], watchStores[i])
		if err := syncers[i].Start(watchCtx); err != nil {
			t.Fatalf("start replica %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		for _, cancel := range cancels {
			cancel()
		}
		for _, syncer := range syncers {
			syncer.Wait()
		}
	})

	delays := make([]time.Duration, 0, 36)
	for round := 0; round < 12; round++ {
		grantID := fmt.Sprintf("grant-%02d", round)
		observed := make(chan time.Duration, len(managers))
		started := time.Now()
		for _, manager := range managers {
			go waitUntilRevoked(manager, tenantID, grantID, started, observed)
		}
		now := time.Now().UTC()
		if _, err := store.PersistRevocation(ctx, security.RevocationRecord{
			TenantID: tenantID, GrantID: grantID, RevokedBy: "user:manager",
			RevokedAt: now, ExpiresAt: now.Add(time.Hour),
		}); err != nil {
			t.Fatalf("persist round %d: %v", round, err)
		}
		for range managers {
			delay := <-observed
			if delay < 0 {
				t.Fatalf("replica did not observe %s before deadline", grantID)
			}
			delays = append(delays, delay)
		}
	}
	sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })
	t.Logf("revocation propagation samples=%d p50=%s p95=%s p99=%s max=%s",
		len(delays), percentile(delays, 0.50), percentile(delays, 0.95),
		percentile(delays, 0.99), delays[len(delays)-1])

	// Replica 3 is offline while this revocation commits.
	cancels[2]()
	syncers[2].Wait()
	now := time.Now().UTC()
	if _, err := store.PersistRevocation(ctx, security.RevocationRecord{
		TenantID: tenantID, GrantID: "grant-during-restart", RevokedBy: "user:manager",
		RevokedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("persist while replica is offline: %v", err)
	}

	restarted := security.NewDelegationManagerWithSecret("integration-secret")
	restartCtx, restartCancel := context.WithCancel(ctx)
	restartSyncer := security.NewRevocationSyncer(restarted, store)
	if err := restartSyncer.Start(restartCtx); err != nil {
		restartCancel()
		t.Fatalf("restart replica from durable snapshot: %v", err)
	}
	if !restarted.IsRevoked(tenantID, "grant-during-restart") || !restarted.IsRevoked(tenantID, "grant-00") {
		restartCancel()
		t.Fatal("restarted replica did not restore active revocations")
	}
	restartCancel()
	restartSyncer.Wait()
}

func waitUntilRevoked(manager *security.DelegationManager, tenantID, grantID string, started time.Time, result chan<- time.Duration) {
	deadline := time.NewTimer(security.RevocationPropagationDeadline)
	ticker := time.NewTicker(time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		if manager.IsRevoked(tenantID, grantID) {
			result <- time.Since(started)
			return
		}
		select {
		case <-deadline.C:
			result <- -1
			return
		case <-ticker.C:
		}
	}
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	index := int(float64(len(values)-1) * quantile)
	return values[index]
}
