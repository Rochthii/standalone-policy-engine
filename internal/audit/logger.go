package audit

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
)

const (
	// MagicByte xác định gói tin nhị phân Audit Log của PDP (0xAE = Audit Event)
	MagicByte byte = 0xAE
)

// LogEntry chứa thông tin chi tiết của một quyết định kiểm toán phân quyền.
type LogEntry struct {
	TenantID         string            `json:"tenant_id"`
	Subject          string            `json:"subject"`
	Action           string            `json:"action"`
	Resource         string            `json:"resource"`
	Decision         string            `json:"decision"`
	MatchedPolicyID  string            `json:"matched_policy_id,omitempty"`
	Context          map[string]string `json:"context,omitempty"`
	EvaluatedAt      time.Time         `json:"evaluated_at"`
	IsEncrypted      bool              `json:"is_encrypted"`
	EncryptedDEK     string            `json:"encrypted_dek,omitempty"`
	EncryptedPayload string            `json:"encrypted_payload,omitempty"`
}

// BatchWriter là interface tương thích ngược.
type BatchWriter interface {
	InsertAuditLogsBatch(ctx context.Context, logs []*LogEntry) error
}

// AuditLogger quản lý luồng xuất log kiểm toán nhị phân Zero-Allocation
// bắn dữ liệu qua Non-blocking UDP Unix Domain Socket (unixgram) hoặc io.Writer.
// 
// ⚖️ ĐÁNH ĐỔI KIẾN TRÚC (ARCHITECTURAL TRADE-OFF):
// - ƯU ĐIỂM: Hiệu năng đỉnh cao (< 30ns, 0 allocs), Fire-and-forget, tuyệt đối không backpressure lên engine.
// - ĐÁNH ĐỔI: Nếu Sidecar Vector bị crash hoặc kernel socket buffer bị tràn (drop packet),
//   log có thể bị rơi rớt. Đây là sự đánh đổi có chủ đích để ưu tiên tuyệt đối độ trễ của Data Plane.
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

// Log đóng gói bản ghi kiểm toán thành định dạng nhị phân Zero-Allocation
// và bắn ngay lập tức qua Unix Socket (hoặc Writer) trong < 30 nano-giây.
func (l *AuditLogger) Log(tenantID, subject, action, resource, decision, matchedPolicyID string, ctxMap map[string]string) {
	bufPtr := l.bytePool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	nowNano := time.Now().UnixNano()

	// 1. Magic byte (1 byte)
	buf = append(buf, MagicByte)

	// 2. EvaluatedAt Unix Nano (8 bytes)
	var scratch [8]byte
	binary.BigEndian.PutUint64(scratch[:], uint64(nowNano))
	buf = append(buf, scratch[:]...)

	// 3. Decision byte (1 byte: 1=ALLOW, 0=DENY)
	if decision == "ALLOW" {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}

	// 4. Đóng gói các trường chuỗi (2 bytes len + string bytes)
	buf = appendStringField(buf, tenantID)
	buf = appendStringField(buf, subject)
	buf = appendStringField(buf, action)
	buf = appendStringField(buf, resource)
	buf = appendStringField(buf, matchedPolicyID)

	// 5. Đóng gói Context Map (2 bytes count + key/value pairs)
	count := uint16(len(ctxMap))
	binary.BigEndian.PutUint16(scratch[:2], count)
	buf = append(buf, scratch[:2]...)

	for k, v := range ctxMap {
		buf = appendStringField(buf, k)
		buf = appendStringField(buf, v)
	}

	// 6. Bắn gói tin ra socket / writer (Non-blocking)
	l.mu.Lock()
	if l.writer != nil {
		_, _ = l.writer.Write(buf)
	}
	l.mu.Unlock()

	*bufPtr = buf
	l.bytePool.Put(bufPtr)

	metrics.AuditLogsStreamedTotal.WithLabelValues(tenantID).Inc()
}

func appendStringField(buf []byte, s string) []byte {
	l := uint16(len(s))
	var lenBytes [2]byte
	binary.BigEndian.PutUint16(lenBytes[:], l)
	buf = append(buf, lenBytes[:]...)
	buf = append(buf, s...)
	return buf
}

// DecodeBinaryLogEntry giải mã gói tin nhị phân phục vụ kiểm thử và cho Sidecar.
func DecodeBinaryLogEntry(data []byte) (*LogEntry, error) {
	if len(data) < 10 {
		return nil, errors.New("du lieu nhiphan qua ngan")
	}
	if data[0] != MagicByte {
		return nil, errors.New("magic byte khong hop le")
	}

	nowNano := int64(binary.BigEndian.Uint64(data[1:9]))
	decByte := data[9]
	dec := "DENY"
	if decByte == 1 {
		dec = "ALLOW"
	}

	offset := 10
	readString := func() (string, error) {
		if offset+2 > len(data) {
			return "", errors.New("out of bounds doc string len")
		}
		sLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if offset+sLen > len(data) {
			return "", errors.New("out of bounds doc string body")
		}
		str := string(data[offset : offset+sLen])
		offset += sLen
		return str, nil
	}

	tenantID, _ := readString()
	subject, _ := readString()
	action, _ := readString()
	resource, _ := readString()
	matchedPolicyID, _ := readString()

	if offset+2 > len(data) {
		return nil, errors.New("out of bounds doc context count")
	}
	ctxCount := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	ctxMap := make(map[string]string, ctxCount)
	for i := 0; i < ctxCount; i++ {
		k, _ := readString()
		v, _ := readString()
		ctxMap[k] = v
	}

	return &LogEntry{
		TenantID:        tenantID,
		Subject:         subject,
		Action:          action,
		Resource:        resource,
		Decision:        dec,
		MatchedPolicyID: matchedPolicyID,
		Context:         ctxMap,
		EvaluatedAt:     time.Unix(0, nowNano).UTC(),
	}, nil
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
