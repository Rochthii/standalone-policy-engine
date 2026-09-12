package audit

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingBatchWriter struct {
	mu      sync.Mutex
	entries []*LogEntry
	written chan struct{}
}

func (w *recordingBatchWriter) InsertAuditLogsBatch(_ context.Context, logs []*LogEntry) error {
	w.mu.Lock()
	w.entries = append(w.entries, logs...)
	w.mu.Unlock()
	select {
	case w.written <- struct{}{}:
	default:
	}
	return nil
}

type blockingBatchWriter struct {
	started chan struct{}
	release chan struct{}
}

func (w *blockingBatchWriter) InsertAuditLogsBatch(ctx context.Context, _ []*LogEntry) error {
	select {
	case w.started <- struct{}{}:
	default:
	}
	select {
	case <-w.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func batchTestConfig() BatchConfig {
	return BatchConfig{
		QueueCapacity: 4,
		BatchSize:     1,
		FlushInterval: time.Hour,
		WriteTimeout:  100 * time.Millisecond,
	}
}

func TestBatchAuditLoggerWritesRedactedEntries(t *testing.T) {
	writer := &recordingBatchWriter{written: make(chan struct{}, 1)}
	logger, err := NewBatchAuditLogger(writer, batchTestConfig())
	if err != nil {
		t.Fatalf("create batch logger: %v", err)
	}
	logger.Start(context.Background())
	t.Cleanup(logger.Stop)

	logger.Log(3, "tenant-a", "user:alice", "READ", "invoice:3", "ALLOW", "policy-3", map[string]string{
		"delegation_proof": "must-not-reach-writer",
		"department":       "Finance",
	})
	select {
	case <-writer.written:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for durable audit write")
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.entries) != 1 {
		t.Fatalf("expected one durable entry, got %d", len(writer.entries))
	}
	entry := writer.entries[0]
	if entry.Context["delegation_proof"] != redactedAuditValue {
		t.Fatal("proof reached durable writer without redaction")
	}
	if entry.Context["department"] != "Finance" {
		t.Fatal("non-sensitive audit context was lost")
	}
}

func TestBatchAuditLoggerDropsWithoutBlockingWhenQueueIsFull(t *testing.T) {
	writer := &blockingBatchWriter{started: make(chan struct{}, 1), release: make(chan struct{})}
	cfg := batchTestConfig()
	cfg.QueueCapacity = 1
	logger, err := NewBatchAuditLogger(writer, cfg)
	if err != nil {
		t.Fatalf("create batch logger: %v", err)
	}
	logger.Start(context.Background())

	logger.Log(1, "tenant-a", "u", "READ", "r", "ALLOW", "p", nil)
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("writer did not receive first batch")
	}
	logger.Log(2, "tenant-a", "u", "READ", "r", "ALLOW", "p", nil)
	logger.Log(3, "tenant-a", "u", "READ", "r", "ALLOW", "p", nil)
	if logger.Stats().Dropped == 0 {
		t.Fatal("full bounded queue must increment dropped counter")
	}

	close(writer.release)
	logger.Stop()
	if logger.Stats().Written != 2 {
		t.Fatalf("expected queued records to flush on stop, stats=%+v", logger.Stats())
	}
}

func TestBatchAuditLoggerBoundsBlockedSink(t *testing.T) {
	writer := &blockingBatchWriter{started: make(chan struct{}, 1), release: make(chan struct{})}
	cfg := batchTestConfig()
	cfg.WriteTimeout = 20 * time.Millisecond
	logger, err := NewBatchAuditLogger(writer, cfg)
	if err != nil {
		t.Fatalf("create batch logger: %v", err)
	}
	logger.Start(context.Background())
	logger.Log(1, "tenant-a", "u", "READ", "r", "ALLOW", "p", nil)
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("writer did not start")
	}

	started := time.Now()
	logger.Stop()
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("shutdown exceeded bounded sink timeout: %s", elapsed)
	}
	stats := logger.Stats()
	if stats.WriteFailures != 1 || stats.Dropped != 1 {
		t.Fatalf("blocked sink failure must be observable: %+v", stats)
	}
}
