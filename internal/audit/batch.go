package audit

import (
	"context"
	"errors"
	"time"

	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
)

// BatchConfig controls the bounded durable-audit queue.
type BatchConfig struct {
	QueueCapacity int
	BatchSize     int
	FlushInterval time.Duration
	WriteTimeout  time.Duration
	SpillDir      string
	SpillMaxBytes int64
	Crypto        *security.EnvelopeCrypto
}

// BatchStats exposes loss and sink-failure counters for readiness/alerting.
type BatchStats struct {
	Queued        uint64
	Written       uint64
	Dropped       uint64
	WriteFailures uint64
	Spilled       uint64
	Replayed      uint64
	SpillFailures uint64
}

func NewBatchAuditLogger(writer BatchWriter, cfg BatchConfig) (*AuditLogger, error) {
	if writer == nil {
		return nil, errors.New("audit batch writer is required")
	}
	if cfg.QueueCapacity <= 0 || cfg.BatchSize <= 0 || cfg.BatchSize > cfg.QueueCapacity {
		return nil, errors.New("audit queue and batch sizes must be positive, with batch <= queue")
	}
	if cfg.FlushInterval <= 0 || cfg.WriteTimeout <= 0 {
		return nil, errors.New("audit flush interval and write timeout must be positive")
	}

	crypto := cfg.Crypto
	if crypto == nil {
		var err error
		crypto, err = security.NewEnvelopeCrypto()
		if err != nil {
			return nil, err
		}
	}
	logger := newBaseAuditLogger(nil)
	logger.crypto = crypto
	if cfg.SpillDir != "" {
		spillStore, err := NewSpillStore(cfg.SpillDir, cfg.SpillMaxBytes, crypto)
		if err != nil {
			return nil, err
		}
		logger.spillStore = spillStore
	}
	logger.batchWriter = writer
	logger.queue = make(chan *LogEntry, cfg.QueueCapacity)
	logger.batchConfig = cfg
	return logger, nil
}

func (l *AuditLogger) enqueue(entry *LogEntry) {
	l.lifecycleMu.RLock()
	defer l.lifecycleMu.RUnlock()
	if l.stopped.Load() {
		l.recordDropped(entry.TenantID, 1)
		return
	}
	select {
	case l.queue <- entry:
		l.queued.Add(1)
	default:
		l.spillOrDrop([]*LogEntry{entry})
	}
}

func (l *AuditLogger) runBatchWorker(ctx context.Context) {
	defer l.workerWG.Done()
	ticker := time.NewTicker(l.batchConfig.FlushInterval)
	defer ticker.Stop()
	batch := make([]*LogEntry, 0, l.batchConfig.BatchSize)
	replay := func() {
		if l.spillStore == nil {
			return
		}
		replayCtx, cancel := context.WithTimeout(context.Background(), l.batchConfig.WriteTimeout)
		count, err := l.spillStore.Replay(replayCtx, l.batchWriter)
		cancel()
		if err != nil {
			l.spillFailures.Add(1)
			metrics.IncrementAuditSpillFailures()
			return
		}
		l.replayed.Add(count)
		metrics.AddAuditLogsReplayed(count)
	}

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), l.batchConfig.WriteTimeout)
		err := l.batchWriter.InsertAuditLogsBatch(ctxWithTimeout, batch)
		cancel()
		if err != nil {
			l.writeFailures.Add(1)
			metrics.IncrementAuditBatchWriteFailures()
			l.spillOrDrop(batch)
		} else {
			l.written.Add(uint64(len(batch)))
			replay()
		}
		batch = batch[:0]
	}
	replay()

	for {
		select {
		case entry := <-l.queue:
			batch = append(batch, entry)
			if len(batch) == l.batchConfig.BatchSize {
				flush()
			}
		case <-ticker.C:
			replay()
			flush()
		case <-ctx.Done():
			for {
				select {
				case entry := <-l.queue:
					batch = append(batch, entry)
					if len(batch) == l.batchConfig.BatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (l *AuditLogger) spillOrDrop(entries []*LogEntry) {
	if l.spillStore != nil && l.spillStore.WriteBatch(entries) == nil {
		l.spilled.Add(uint64(len(entries)))
		for _, entry := range entries {
			metrics.IncrementAuditLogsSpilled(entry.TenantID)
		}
		return
	}
	l.spillFailures.Add(1)
	metrics.IncrementAuditSpillFailures()
	for _, entry := range entries {
		l.recordDropped(entry.TenantID, 1)
	}
}

func (l *AuditLogger) recordDropped(tenantID string, count uint64) {
	l.dropped.Add(count)
	metrics.AddAuditLogsDropped(tenantID, count)
}

func (l *AuditLogger) Stats() BatchStats {
	return BatchStats{
		Queued:        l.queued.Load(),
		Written:       l.written.Load(),
		Dropped:       l.dropped.Load(),
		WriteFailures: l.writeFailures.Load(),
		Spilled:       l.spilled.Load(),
		Replayed:      l.replayed.Load(),
		SpillFailures: l.spillFailures.Load(),
	}
}
