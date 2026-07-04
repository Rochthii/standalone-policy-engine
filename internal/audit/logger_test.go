package audit

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

type mockBatchWriter struct {
	mu    sync.Mutex
	err   error
	calls int
	logs  []*LogEntry
}

func (m *mockBatchWriter) InsertAuditLogsBatch(ctx context.Context, logs []*LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.logs = append(m.logs, logs...)
	return m.err
}

func TestAuditLogger_SpillToDiskAndSizeLimit(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "audit_spill_test")
	if err != nil {
		t.Fatalf("Không thể tạo temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	writer := &mockBatchWriter{}
	// Khởi tạo logger với buffer size cực nhỏ = 1, dung lượng đĩa tối đa 150 bytes
	l := NewAuditLogger(writer, tempDir, 1)
	l.maxSpillDirSize = 250

	// Ghi 3 logs trực tiếp xuống SSD bằng spillToDisk
	// Đảm bảo thời gian khác nhau để thứ tự ModTime khác biệt rõ ràng và được ghi vào các file khác nhau
	entry1 := &LogEntry{TenantID: "t1", Subject: "u1", Action: "a1", Resource: "r1", Decision: "ALLOW", EvaluatedAt: time.Now().Add(-48 * time.Hour)}
	entry2 := &LogEntry{TenantID: "t2", Subject: "u2", Action: "a2", Resource: "r2", Decision: "DENY", EvaluatedAt: time.Now().Add(-24 * time.Hour)}
	entry3 := &LogEntry{TenantID: "t3", Subject: "u3", Action: "a3", Resource: "r3", Decision: "ALLOW", EvaluatedAt: time.Now()}

	l.spillToDisk(entry1)
	l.spillToDisk(entry2)
	l.spillToDisk(entry3)

	// Vì maxSpillDirSize = 150 bytes (nhỏ hơn kích thước của 3 entry gộp lại), 
	// hệ thống bắt buộc phải kích hoạt enforceSizeLimit() tự động xóa bớt file cũ nhất.
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Đọc thư mục spill thất bại: %v", err)
	}

	// Đảm bảo thư mục không trống và kích thước tổng nhỏ hơn 150 bytes
	totalSize, err := l.getDirSize()
	if err != nil {
		t.Fatalf("Lấy kích thước thư mục thất bại: %v", err)
	}

	if totalSize > l.maxSpillDirSize {
		t.Errorf("Tổng dung lượng (%d bytes) vượt quá giới hạn cấu hình (%d bytes)", totalSize, l.maxSpillDirSize)
	}

	t.Logf("Tổng dung lượng thực tế sau giới hạn: %d bytes, Số lượng file log: %d", totalSize, len(files))
}

func TestAuditLogger_ReplayBackoff(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "audit_backoff_test")
	if err != nil {
		t.Fatalf("Không thể tạo temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// DB bị sập
	writer := &mockBatchWriter{err: errors.New("PostgreSQL connection refused")}
	l := NewAuditLogger(writer, tempDir, 10)

	// Ghi 1 log vào file spill
	entry := &LogEntry{TenantID: "t1", Subject: "u1", Action: "a1", Resource: "r1", Decision: "ALLOW", EvaluatedAt: time.Now()}
	l.spillToDisk(entry)

	// Chạy thử replay lần đầu -> bị lỗi
	ctx := context.Background()
	errReplay := l.replayLogs(ctx)
	if errReplay == nil {
		t.Fatal("Kỳ vọng lỗi khi DB sập nhưng không gặp lỗi")
	}

	// Xác nhận file log thô vẫn còn lưu trên SSD
	files, _ := ioutil.ReadDir(tempDir)
	if len(files) == 0 {
		t.Error("File log thô không được bị xóa khi DB gặp lỗi")
	}

	// DB phục hồi
	writer.mu.Lock()
	writer.err = nil
	writer.mu.Unlock()

	// Chạy replay tiếp theo -> thành công
	errReplay = l.replayLogs(ctx)
	if errReplay != nil {
		t.Fatalf("Kỳ vọng thành công sau khi DB phục hồi, thực tế lỗi: %v", errReplay)
	}

	// Xác nhận file log thô đã được xóa sau khi đồng bộ thành công
	files, _ = ioutil.ReadDir(tempDir)
	if len(files) > 0 {
		t.Error("File log thô phải được xóa sau khi đồng bộ thành công")
	}
}
