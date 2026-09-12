package audit

import (
	"context"
	"errors"
	"time"

	"standalone-policy-engine/internal/metrics"
)

// BatchConfig controls the bounded durable-audit queue.
type BatchConfig struct {
	QueueCapacity int
	BatchSize     int
	FlushInterval time.Duration
	WriteTimeout  time.Duration
}

// BatchStats exposes loss and sink-failure counters for readiness/alerting.
type BatchStats struct {
	Queued        uint64
	Written       uint64
	Dropped       uint64
	WriteFailures uint64
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

	logger := newBaseAuditLogger(nil)
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
		l.recordDropped(entry.TenantID, 1)
	}
}

func (l *AuditLogger) runBatchWorker(ctx context.Context) {
	defer l.workerWG.Done()
	ticker := time.NewTicker(l.batchConfig.FlushInterval)
	defer ticker.Stop()
	batch := make([]*LogEntry, 0, l.batchConfig.BatchSize)

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
			for _, entry := range batch {
				l.recordDropped(entry.TenantID, 1)
			}
		} else {
			l.written.Add(uint64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-l.queue:
			batch = append(batch, entry)
			if len(batch) == l.batchConfig.BatchSize {
				flush()
			}
		case <-ticker.C:
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
	}
}
