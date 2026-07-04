package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
	"strings"
	"sync"
	"time"
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

// BatchWriter là interface giúp ghi log theo lô xuống PostgreSQL.
// Tránh import cycle giữa package audit và storage.
type BatchWriter interface {
	InsertAuditLogsBatch(ctx context.Context, logs []*LogEntry) error
}

// AuditLogger quản lý hàng đợi log kiểm toán bất đồng bộ (Ring Buffer)
// và cơ chế tự phục hồi Spill-to-Disk khi database quá tải.
type AuditLogger struct {
	writer          BatchWriter
	spillDir        string
	logChan         chan *LogEntry
	stopChan        chan struct{}
	crypto          *security.EnvelopeCrypto
	maxSpillDirSize int64

	fileMutex sync.Mutex
	wg        sync.WaitGroup
}

// NewAuditLogger khởi tạo một instance AuditLogger.
func NewAuditLogger(writer BatchWriter, spillDir string, bufferSize int) *AuditLogger {
	// Tạo thư mục spill logs nếu chưa có
	if err := os.MkdirAll(spillDir, 0755); err != nil {
		// Log warning
	}

	crypto, err := security.NewEnvelopeCrypto()
	if err != nil {
		// Log warning
	}

	return &AuditLogger{
		writer:          writer,
		spillDir:        spillDir,
		logChan:         make(chan *LogEntry, bufferSize),
		stopChan:        make(chan struct{}),
		crypto:          crypto,
		maxSpillDirSize: 1024 * 1024 * 1024, // Mặc định 1 GB
	}
}

// Log đẩy log kiểm toán vào Ring Buffer một cách bất đồng bộ.
// Nếu buffer bị đầy (Postgres nghẽn), tự động chuyển hướng ghi log xuống SSD cục bộ (Spill-to-Disk).
func (l *AuditLogger) Log(tenantID, subject, action, resource, decision, matchedPolicyID string, ctxMap map[string]string) {
	entry := &LogEntry{
		TenantID:        tenantID,
		Subject:         subject,
		Action:          action,
		Resource:        resource,
		Decision:        decision,
		MatchedPolicyID: matchedPolicyID,
		Context:         ctxMap,
		EvaluatedAt:     time.Now(),
	}

	// Thực hiện mã hóa Envelope các trường nhạy cảm
	if l.crypto != nil {
		payloadMap := map[string]interface{}{
			"subject":  subject,
			"action":   action,
			"resource": resource,
			"context":  ctxMap,
		}
		payloadBytes, err := json.Marshal(payloadMap)
		if err == nil {
			ciphertext, encDEK, err := l.crypto.Encrypt(payloadBytes)
			if err == nil {
				entry.IsEncrypted = true
				entry.EncryptedPayload = ciphertext
				entry.EncryptedDEK = encDEK
				// Xoá thông tin thô nhạy cảm để bảo mật bộ nhớ RAM và log spill
				entry.Subject = ""
				entry.Action = ""
				entry.Resource = ""
				entry.Context = nil
			}
		}
	}

	select {
	case l.logChan <- entry:
		// Đẩy vào hàng đợi thành công
	default:
		// Hàng đợi Ring Buffer bị đầy -> Kích hoạt Spill-to-Disk bảo toàn log và giải phóng gRPC thread
		l.spillToDisk(entry)
	}
}

// Start khởi chạy worker pool nền thu gom log ghi DB và luồng replay log thô.
func (l *AuditLogger) Start(ctx context.Context) {
	l.wg.Add(2)

	// Worker 1: Thu thập log từ channel và Batch Insert vào DB
	go l.worker(ctx)

	// Worker 2: Định kỳ kiểm tra file log thô trên SSD và ghi đè lại DB khi DB hoạt động bình thường
	go l.replayWorker(ctx)
}

// Stop dừng an toàn AuditLogger, đảm bảo flush toàn bộ log còn lại trong buffer.
func (l *AuditLogger) Stop() {
	close(l.stopChan)
	l.wg.Wait()

	// Thu gom nốt số log còn sót lại trong channel ghi xuống SSD trước khi tắt hẳn
	close(l.logChan)
	for entry := range l.logChan {
		l.spillToDisk(entry)
	}
}

func (l *AuditLogger) worker(ctx context.Context) {
	defer l.wg.Done()

	batch := make([]*LogEntry, 0, 100)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChan:
			// Flush dữ liệu hiện tại trước khi thoát
			if len(batch) > 0 {
				l.flushBatch(ctx, batch)
			}
			return

		case entry := <-l.logChan:
			batch = append(batch, entry)
			if len(batch) >= 100 {
				l.flushBatch(ctx, batch)
				batch = make([]*LogEntry, 0, 100)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				l.flushBatch(ctx, batch)
				batch = make([]*LogEntry, 0, 100)
			}
		}
	}
}

func (l *AuditLogger) flushBatch(ctx context.Context, batch []*LogEntry) {
	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err := l.writer.InsertAuditLogsBatch(dbCtx, batch)
	if err != nil {
		// PostgreSQL sập -> Chuyển hướng toàn bộ batch xuống SSD
		for _, entry := range batch {
			l.spillToDisk(entry)
		}
	}
}

// spillToDisk ghi log dạng JSON Lines xuống file SSD vật lý cục bộ.
func (l *AuditLogger) spillToDisk(entry *LogEntry) {
	l.fileMutex.Lock()
	defer l.fileMutex.Unlock()

	dateStr := entry.EvaluatedAt.Format("2006-01-02")
	filePath := filepath.Join(l.spillDir, fmt.Sprintf("spill_%s.log", dateStr))

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err == nil {
		metrics.IncrementAuditLogsSpilled(entry.TenantID)
	}

	// Đảm bảo không vượt quá giới hạn dung lượng đĩa tối đa
	l.enforceSizeLimit()
}

// enforceSizeLimit kiểm tra và xóa bớt file log cũ nhất nếu tổng dung lượng spill logs vượt quá 1 GB.
func (l *AuditLogger) enforceSizeLimit() {
	size, err := l.getDirSize()
	if err != nil || size <= l.maxSpillDirSize {
		return
	}

	files, err := ioutil.ReadDir(l.spillDir)
	if err != nil || len(files) == 0 {
		return
	}

	var oldestFile os.FileInfo
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "spill_") {
			continue
		}
		if oldestFile == nil || file.ModTime().Before(oldestFile.ModTime()) {
			oldestFile = file
		}
	}

	if oldestFile != nil {
		_ = os.Remove(filepath.Join(l.spillDir, oldestFile.Name()))
		// Gọi đệ quy cho đến khi dung lượng nhỏ hơn 1 GB
		l.enforceSizeLimit()
	}
}

// getDirSize tính tổng dung lượng thư mục spill logs.
func (l *AuditLogger) getDirSize() (int64, error) {
	var size int64
	err := filepath.Walk(l.spillDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// replayWorker chạy ngầm định kỳ, đọc log từ SSD và đồng bộ lại vào DB với cơ chế exponential back-off.
func (l *AuditLogger) replayWorker(ctx context.Context) {
	defer l.wg.Done()

	backoff := 1 * time.Second
	maxBackoff := 1 * time.Minute

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	for {
		select {
		case <-l.stopChan:
			return
		case <-timer.C:
			err := l.replayLogs(ctx)
			if err != nil {
				// Tăng thời gian back-off khi gặp lỗi DB
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			} else {
				// Reset về chu kỳ bình thường 5 giây khi thành công
				backoff = 5 * time.Second
			}
			timer.Reset(backoff)
		}
	}
}

// replayLogs đồng bộ log từ SSD vào DB và trả về lỗi nếu quá trình insert thất bại.
func (l *AuditLogger) replayLogs(ctx context.Context) error {
	l.fileMutex.Lock()
	files, err := ioutil.ReadDir(l.spillDir)
	l.fileMutex.Unlock()

	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	// Xử lý từng file
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "spill_") {
			continue
		}

		filePath := filepath.Join(l.spillDir, file.Name())

		// Đọc nội dung file
		l.fileMutex.Lock()
		content, err := ioutil.ReadFile(filePath)
		l.fileMutex.Unlock()
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		batch := make([]*LogEntry, 0)

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			entry := &LogEntry{}
			if err := json.Unmarshal([]byte(line), entry); err == nil {
				batch = append(batch, entry)
			}
		}

		if len(batch) == 0 {
			l.fileMutex.Lock()
			_ = os.Remove(filePath)
			l.fileMutex.Unlock()
			continue
		}

		// Thử ghi lại vào DB
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = l.writer.InsertAuditLogsBatch(dbCtx, batch)
		cancel()

		if err == nil {
			// Đồng bộ thành công -> Xóa file log thô
			l.fileMutex.Lock()
			_ = os.Remove(filePath)
			l.fileMutex.Unlock()
		} else {
			// DB vẫn lỗi -> Trả về lỗi để kích hoạt back-off
			return err
		}
	}
	return nil
}
