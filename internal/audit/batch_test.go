package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"standalone-policy-engine/internal/security"
)

const (
	testOldAuditKEK = "test-audit-kek-old-32-bytes-key!"
	testNewAuditKEK = "test-audit-kek-new-32-bytes-key!"
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

type failingBatchWriter struct{}

func (failingBatchWriter) InsertAuditLogsBatch(context.Context, []*LogEntry) error {
	return errors.New("database unavailable")
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
	payload, err := VerifyAndDecryptEntry(logger.crypto, entry)
	if err != nil {
		t.Fatalf("decrypt durable audit entry: %v", err)
	}
	if payload.Context["delegation_proof"] != redactedAuditValue {
		t.Fatal("proof reached durable writer without redaction")
	}
	if payload.Context["department"] != "Finance" {
		t.Fatal("non-sensitive audit context was lost")
	}
	if !entry.IsEncrypted || entry.Subject != "" || entry.Action != "" || entry.Resource != "" || entry.Context != nil {
		t.Fatal("durable writer received plaintext audit fields")
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

func TestAuditSpillReplayAcrossRestartAndKeyRotation(t *testing.T) {
	spillDir := t.TempDir()
	keys := map[string]string{"old": testOldAuditKEK, "new": testNewAuditKEK}
	oldCrypto, err := security.NewEnvelopeCryptoWithKeyring("old", keys)
	if err != nil {
		t.Fatal(err)
	}
	cfg := batchTestConfig()
	cfg.Crypto = oldCrypto
	cfg.SpillDir = spillDir
	cfg.SpillMaxBytes = 1 << 20

	failedLogger, err := NewBatchAuditLogger(failingBatchWriter{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	failedLogger.Start(context.Background())
	failedLogger.Log(9, "tenant-a", "user:alice", "READ", "invoice:9", "ALLOW", "", map[string]string{"request_id": "req-9"})
	failedLogger.Stop()
	if stats := failedLogger.Stats(); stats.Spilled != 1 || stats.Dropped != 0 {
		t.Fatalf("failed write must spill without loss: %+v", stats)
	}

	newCrypto, err := security.NewEnvelopeCryptoWithKeyring("new", keys)
	if err != nil {
		t.Fatal(err)
	}
	writer := &recordingBatchWriter{written: make(chan struct{}, 2)}
	cfg.Crypto = newCrypto
	restartedLogger, err := NewBatchAuditLogger(writer, cfg)
	if err != nil {
		t.Fatal(err)
	}
	restartedLogger.Start(context.Background())
	select {
	case <-writer.written:
	case <-time.After(time.Second):
		t.Fatal("restart did not replay spilled audit entry")
	}

	restartedLogger.Log(10, "tenant-a", "user:bob", "WRITE", "invoice:10", "DENY", "", nil)
	select {
	case <-writer.written:
	case <-time.After(time.Second):
		t.Fatal("rotated logger did not write active-key entry")
	}
	restartedLogger.Stop()

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.entries) != 2 || writer.entries[0].KeyID != "old" || writer.entries[1].KeyID != "new" {
		t.Fatalf("unexpected key lifecycle entries: %#v", writer.entries)
	}
	oldPayload, err := VerifyAndDecryptEntry(newCrypto, writer.entries[0])
	if err != nil || oldPayload.Subject != "user:alice" {
		t.Fatalf("rotated key ring could not replay old record: payload=%#v err=%v", oldPayload, err)
	}
	files, err := os.ReadDir(spillDir)
	if err != nil || len(files) != 0 {
		t.Fatalf("successfully replayed spill files must be removed: files=%v err=%v", files, err)
	}
}

func TestAuditSpillRejectsTamperedMetadata(t *testing.T) {
	crypto, err := security.NewEnvelopeCryptoWithKeyring("old", map[string]string{"old": testOldAuditKEK})
	if err != nil {
		t.Fatal(err)
	}
	entry := &LogEntry{
		Timestamp: time.Now().UnixNano(), TenantID: "tenant-a", Subject: "user:alice",
		Action: "READ", Resource: "invoice:1", Decision: "ALLOW",
	}
	if err := sealAuditEntry(crypto, entry); err != nil {
		t.Fatal(err)
	}
	store, err := NewSpillStore(t.TempDir(), 1<<20, crypto)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteBatch([]*LogEntry{entry}); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(store.dir)
	path := filepath.Join(store.dir, files[0].Name())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `"tenant_id":"tenant-a"`, `"tenant_id":"tenant-b"`, 1)
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	writer := &recordingBatchWriter{written: make(chan struct{}, 1)}
	if _, err := store.Replay(context.Background(), writer); err == nil || !strings.Contains(err.Error(), "tampered") {
		t.Fatalf("tampered spill must be rejected, got %v", err)
	}
	if len(writer.entries) != 0 {
		t.Fatal("tampered spill reached durable writer")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("tampered spill must remain for operator investigation")
	}
}
