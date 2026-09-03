package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
)

// LogEntry chứa thông tin chi tiết của một quyết định kiểm toán phân quyền.
type LogEntry struct {
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
	EncryptedDEK     string            `json:"encrypted_dek,omitempty"`
	EncryptedPayload string            `json:"encrypted_payload,omitempty"`
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
}

// NewAuditLogger khởi tạo AuditLogger tương thích ngược (ghi ra os.Stdout).
func NewAuditLogger(writer BatchWriter, spillDir string, bufferSize int) *AuditLogger {
	return NewStreamAuditLogger(os.Stdout)
}

// NewStreamAuditLogger khởi tạo AuditLogger ghi nhị phân ra một io.Writer bất kỳ.
func NewStreamAuditLogger(w io.Writer) *AuditLogger {
	if w == nil {
		w = os.Stdout
	}
	crypto, _ := security.NewEnvelopeCrypto()

	return &AuditLogger{
		writer: w,
		crypto: crypto,
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

	crypto, _ := security.NewEnvelopeCrypto()

	return &AuditLogger{
		conn:   conn,
		writer: conn,
		crypto: crypto,
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
			buf = escapeJSON(buf, v)
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

// escapeJSON mã hóa an toàn các ký tự đặc biệt cho chuỗi JSON mà không cấp phát bộ nhớ heap.
func escapeJSON(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

// DecodeNDJSONLogEntry giải mã gói tin JSON Lines phục vụ kiểm thử và debug.
func DecodeNDJSONLogEntry(data []byte) (*LogEntry, error) {
	if len(data) == 0 {
		return nil, errors.New("du lieu ndjson rong")
	}

	var entry LogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	if entry.Timestamp > 0 {
		entry.EvaluatedAt = time.Unix(0, entry.Timestamp).UTC()
	}

	return &entry, nil
}

// DecodeBinaryLogEntry tương thích ngược (chuyển hướng sang DecodeNDJSONLogEntry).
func DecodeBinaryLogEntry(data []byte) (*LogEntry, error) {
	return DecodeNDJSONLogEntry(data)
}

// Start tương thích ngược.
func (l *AuditLogger) Start(ctx context.Context) {}

// Stop đóng kết nối và dừng AuditLogger.
func (l *AuditLogger) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		_ = l.conn.Close()
	}
	select {
	case <-l.stopChan:
		return
	default:
		close(l.stopChan)
	}
}
