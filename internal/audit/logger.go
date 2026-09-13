package audit

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
)

// LogEntry chứa thông tin chi tiết của một quyết định kiểm toán phân quyền.
type LogEntry struct {
	AuditID          string            `json:"audit_id"`
	PayloadVersion   int               `json:"payload_version"`
	Timestamp        int64             `json:"ts"`
	RevisionID       uint64            `json:"rev"`
	TenantID         string            `json:"tenant_id"`
	Subject          string            `json:"subject"`
	Action           string            `json:"action"`
	Resource         string            `json:"resource"`
	Decision         string            `json:"decision"`
	MatchedPolicyID  string            `json:"matched_policy_id,omitempty"`
	Context          map[string]string `json:"context,omitempty"`
	EvaluatedAt      time.Time         `json:"evaluated_at,omitempty"`
	IsEncrypted      bool              `json:"is_encrypted,omitempty"`
	KeyID            string            `json:"key_id,omitempty"`
	RequestID        string            `json:"request_id,omitempty"`
	TraceID          string            `json:"trace_id,omitempty"`
	EncryptedDEK     string            `json:"encrypted_dek,omitempty"`
	EncryptedPayload string            `json:"encrypted_payload,omitempty"`
	IntegrityTag     string            `json:"integrity_tag,omitempty"`
}

// BatchWriter là interface tương thích ngược.
type BatchWriter interface {
	InsertAuditLogsBatch(ctx context.Context, logs []*LogEntry) error
}

// AuditLogger quản lý luồng xuất log kiểm toán NDJSON Zero-Allocation
// bắn dữ liệu qua Non-blocking UDP Unix Domain Socket (unixgram) hoặc io.Writer.
//
// ⚖️ THIẾT KẾ KIẾN TRÚC CLOUD-NATIVE:
//   - Đạt 0 Heap Allocations trên Hot-Path thông qua sync.Pool byte slice.
//   - Định dạng Newline Delimited JSON (NDJSON) tương thích 100% với Vector Sidecar,
//     Kubernetes stdout, FluentBit và ClickHouse.
//   - Bổ sung trường RevisionID để phục vụ truy vết Eventual Consistency.
type AuditLogger struct {
	writer   io.Writer
	conn     net.Conn
	mu       sync.Mutex
	crypto   *security.EnvelopeCrypto
	bytePool sync.Pool
	stopChan chan struct{}

	batchWriter   BatchWriter
	queue         chan *LogEntry
	batchConfig   BatchConfig
	workerWG      sync.WaitGroup
	startOnce     sync.Once
	stopOnce      sync.Once
	cancel        context.CancelFunc
	lifecycleMu   sync.RWMutex
	stopped       atomic.Bool
	queued        atomic.Uint64
	written       atomic.Uint64
	dropped       atomic.Uint64
	writeFailures atomic.Uint64
	spilled       atomic.Uint64
	replayed      atomic.Uint64
	spillFailures atomic.Uint64
	spillStore    *SpillStore
}

// NewAuditLogger khởi tạo AuditLogger tương thích ngược (ghi ra os.Stdout).
func NewAuditLogger(writer BatchWriter, spillDir string, bufferSize int) *AuditLogger {
	logger, err := NewBatchAuditLogger(writer, BatchConfig{
		QueueCapacity: bufferSize,
		BatchSize:     min(bufferSize, 128),
		FlushInterval: 100 * time.Millisecond,
		WriteTimeout:  2 * time.Second,
		SpillDir:      spillDir,
		SpillMaxBytes: 1 << 30,
	})
	if err != nil {
		return NewStreamAuditLogger(os.Stdout)
	}
	return logger
}

// NewStreamAuditLogger khởi tạo AuditLogger ghi nhị phân ra một io.Writer bất kỳ.
func NewStreamAuditLogger(w io.Writer) *AuditLogger {
	if w == nil {
		w = os.Stdout
	}
	return newBaseAuditLogger(w)
}

func newBaseAuditLogger(w io.Writer) *AuditLogger {
	return &AuditLogger{
		writer: w,
		bytePool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 0, 1024)
				return &b
			},
		},
		stopChan: make(chan struct{}),
	}
}

// NewUnixgramAuditLogger khởi tạo AuditLogger kết nối qua Non-blocking UDP Unix Domain Socket.
func NewUnixgramAuditLogger(sockPath string) (*AuditLogger, error) {
	raddr, err := net.ResolveUnixAddr("unixgram", sockPath)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUnix("unixgram", nil, raddr)
	if err != nil {
		return nil, err
	}

	return &AuditLogger{
		conn:   conn,
		writer: conn,
		bytePool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 0, 1024)
				return &b
			},
		},
		stopChan: make(chan struct{}),
	}, nil
}

// Log đóng gói bản ghi kiểm toán thành JSON Lines (NDJSON) Zero-Allocation
// và bắn ngay lập tức qua Unix Socket (hoặc Writer) mà không sinh rác GC.
func (l *AuditLogger) Log(revisionID uint64, tenantID, subject, action, resource, decision, matchedPolicyID string, ctxMap map[string]string) {
	if l.batchWriter != nil {
		entry := &LogEntry{
			Timestamp:       time.Now().UnixNano(),
			RevisionID:      revisionID,
			TenantID:        tenantID,
			Subject:         subject,
			Action:          action,
			Resource:        resource,
			Decision:        decision,
			MatchedPolicyID: matchedPolicyID,
			Context:         redactedAuditContext(ctxMap),
		}
		if err := sealAuditEntry(l.crypto, entry); err != nil {
			l.recordDropped(tenantID, 1)
			return
		}
		l.enqueue(entry)
		return
	}
	bufPtr := l.bytePool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	nowNano := time.Now().UnixNano()

	// 1. Khởi tạo JSON Object và các trường chính
	buf = append(buf, `{"ts":`...)
	buf = strconv.AppendInt(buf, nowNano, 10)
	buf = append(buf, `,"rev":`...)
	buf = strconv.AppendUint(buf, revisionID, 10)
	buf = append(buf, `,"tenant_id":"`...)
	buf = escapeJSON(buf, tenantID)
	buf = append(buf, `","subject":"`...)
	buf = escapeJSON(buf, subject)
	buf = append(buf, `","action":"`...)
	buf = escapeJSON(buf, action)
	buf = append(buf, `","resource":"`...)
	buf = escapeJSON(buf, resource)
	buf = append(buf, `","decision":"`...)
	buf = append(buf, decision...)
	buf = append(buf, `","matched_policy_id":"`...)
	buf = escapeJSON(buf, matchedPolicyID)
	buf = append(buf, '"')

	// 2. Đóng gói Context Map nếu có
	if len(ctxMap) > 0 {
		buf = append(buf, `,"context":{`...)
		first := true
		for k, v := range ctxMap {
			if !first {
				buf = append(buf, ',')
			}
			first = false
			buf = append(buf, '"')
			buf = escapeJSON(buf, k)
			buf = append(buf, `":"`...)
			if shouldRedactAuditContextKey(k) {
				buf = append(buf, redactedAuditValue...)
			} else {
				buf = escapeJSON(buf, v)
			}
			buf = append(buf, '"')
		}
		buf = append(buf, '}')
	}

	// 3. Kết thúc dòng NDJSON
	buf = append(buf, "}\n"...)

	// 4. Ghi gói tin ra Socket / Writer (Non-blocking)
	l.mu.Lock()
	if l.writer != nil {
		_, _ = l.writer.Write(buf)
	}
	l.mu.Unlock()

	*bufPtr = buf
	l.bytePool.Put(bufPtr)

	metrics.AuditLogsStreamedTotal.WithLabelValues(tenantID).Inc()
}

// Start tương thích ngược.
func (l *AuditLogger) Start(ctx context.Context) {
	if l.batchWriter == nil {
		return
	}
	l.startOnce.Do(func() {
		workerCtx, cancel := context.WithCancel(ctx)
		l.cancel = cancel
		l.workerWG.Add(1)
		go l.runBatchWorker(workerCtx)
	})
}

// Stop đóng kết nối và dừng AuditLogger.
func (l *AuditLogger) Stop() {
	l.stopOnce.Do(func() {
		l.lifecycleMu.Lock()
		l.stopped.Store(true)
		if l.cancel != nil {
			l.cancel()
		}
		l.lifecycleMu.Unlock()
		l.workerWG.Wait()

		l.mu.Lock()
		defer l.mu.Unlock()
		if l.conn != nil {
			_ = l.conn.Close()
		}
		close(l.stopChan)
	})
}
