package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"standalone-policy-engine/internal/metrics"
	"sync"
	"time"
)

// LogEntry chứa thông tin chi tiết của một quyết định kiểm toán phân quyền.
type LogEntry struct {
	TenantID        string            `json:"tenant_id"`
	Subject         string            `json:"subject"`
	Action          string            `json:"action"`
	Resource        string            `json:"resource"`
	Decision        string            `json:"decision"`
	MatchedPolicyID string            `json:"matched_policy_id,omitempty"`
	Context         map[string]string `json:"context,omitempty"`
	EvaluatedAt     time.Time         `json:"evaluated_at"`
}

// BatchWriter là interface giúp ghi log theo lô xuống PostgreSQL.
// Tránh import cycle giữa package audit và storage.
type BatchWriter interface {
	InsertAuditLogsBatch(ctx context.Context, logs []*LogEntry) error
}

// AuditLogger quản lý hàng đợi log kiểm toán bất đồng bộ (Ring Buffer)
// và cơ chế tự phục hồi Spill-to-Disk khi database quá tải.
type AuditLogger struct {
	writer   BatchWriter
	spillDir string
	logChan  chan *LogEntry
	stopChan chan struct{}

	fileMutex sync.Mutex
	wg        sync.WaitGroup
}

// NewAuditLogger khởi tạo một instance AuditLogger.
func NewAuditLogger(writer BatchWriter, spillDir string, bufferSize int) *AuditLogger {
	// Tạo thư mục spill logs nếu chưa có
	if err := os.MkdirAll(spillDir, 0755); err != nil {
		// Log warning
	}

	return &AuditLogger{
		writer:   writer,
		spillDir: spillDir,
		logChan:  make(chan *LogEntry, bufferSize),
		stopChan: make(chan struct{}),
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
}

// replayWorker chạy ngầm định kỳ mỗi 5 giây, đọc log từ SSD và đồng bộ lại vào DB.
func (l *AuditLogger) replayWorker(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChan:
			return
		case <-ticker.C:
			l.replayLogs(ctx)
		}
	}
}

func (l *AuditLogger) replayLogs(ctx context.Context) {
	l.fileMutex.Lock()
	files, err := ioutil.ReadDir(l.spillDir)
	l.fileMutex.Unlock()

	if err != nil || len(files) == 0 {
		return
	}

	// Xử lý từng file
	for _, file := range files {
		if file.IsDir() || !filepath.HasPrefix(file.Name(), "spill_") {
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
			_ = os.Remove(filePath)
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
			// DB vẫn lỗi -> Dừng tiến trình replay cho lượt tiếp theo
			break
		}
	}
}

// import strings for file check
import "strings"
