package security

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// RevocationPropagationDeadline is the release SLO for a healthy PostgreSQL
// path, including notification delivery and in-memory cache application.
const RevocationPropagationDeadline = 5 * time.Second

// RevocationSyncer keeps the in-memory O(1) lookup cache synchronized with
// the durable store. Startup does not complete until the first snapshot is
// installed, preventing an uninitialized replica from serving requests.
type RevocationSyncer struct {
	manager     *DelegationManager
	store       RevocationStore
	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewRevocationSyncer(manager *DelegationManager, store RevocationStore) *RevocationSyncer {
	return &RevocationSyncer{manager: manager, store: store}
}

func (s *RevocationSyncer) Start(ctx context.Context) error {
	if s == nil || s.manager == nil || s.store == nil {
		return fmt.Errorf("revocation syncer is not configured")
	}
	workerCtx, cancel := context.WithCancel(ctx)
	s.manager.SetRevocationReady(false)
	s.lifecycleMu.Lock()
	s.cancel = cancel
	s.lifecycleMu.Unlock()
	ready := make(chan error, 1)
	s.wg.Add(1)
	go s.run(workerCtx, ready)
	timer := time.NewTimer(RevocationPropagationDeadline)
	defer timer.Stop()
	select {
	case err := <-ready:
		if err != nil {
			cancel()
			s.wg.Wait()
		}
		return err
	case <-ctx.Done():
		cancel()
		s.wg.Wait()
		return ctx.Err()
	case <-timer.C:
		cancel()
		s.wg.Wait()
		return fmt.Errorf("durable revocation snapshot exceeded %s", RevocationPropagationDeadline)
	}
}

func (s *RevocationSyncer) Wait() {
	if s != nil {
		s.wg.Wait()
	}
}

func (s *RevocationSyncer) Stop() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	cancel := s.cancel
	s.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *RevocationSyncer) run(ctx context.Context, ready chan<- error) {
	defer s.wg.Done()
	initialized := false
	for {
		s.manager.SetRevocationReady(false)
		err := s.store.WatchRevocations(ctx, func(records []RevocationRecord) {
			for _, record := range records {
				s.manager.ApplyRevocation(record)
			}
			s.manager.SetRevocationReady(true)
			if !initialized {
				initialized = true
				ready <- nil
			}
		}, func(record RevocationRecord) {
			s.manager.ApplyRevocation(record)
		})
		if ctx.Err() != nil {
			s.manager.SetRevocationReady(false)
			if !initialized {
				ready <- ctx.Err()
			}
			return
		}
		if !initialized {
			ready <- fmt.Errorf("initialize durable revocations: %w", err)
			return
		}
		s.manager.SetRevocationReady(false)
		log.Printf("[Security] revocation watch interrupted: %v; reconnecting", err)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
