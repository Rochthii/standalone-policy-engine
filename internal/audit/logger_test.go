package audit

import (
	"bytes"
	"sync"
	"testing"
)

type noopWriter struct{}

func (n *noopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestAuditLogger_NDJSONPackagingAndDecoding(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewStreamAuditLogger(buf)
	defer logger.Stop()

	ctxMap := map[string]string{
		"ip":     "192.168.1.100",
		"device": "ios_secure",
	}

	revisionID := uint64(42)
	logger.Log(revisionID, "tenant-123", "user:alice", "READ", "file:secret.pdf", "ALLOW", "P-ALLOW-1", ctxMap)

	if buf.Len() == 0 {
		t.Fatal("Buffer không được rỗng sau khi ghi log")
	}

	// Đảm bảo dòng kết thúc bằng newline (NDJSON)
	rawBytes := buf.Bytes()
	if rawBytes[len(rawBytes)-1] != '\n' {
		t.Errorf("Dòng NDJSON phải kết thúc bằng newline '\\n'")
	}

	entry, err := DecodeNDJSONLogEntry(rawBytes)
	if err != nil {
		t.Fatalf("Decode NDJSON log entry lỗi: %v (raw: %s)", err, string(rawBytes))
	}

	if entry.RevisionID != 42 {
		t.Errorf("RevisionID không khớp: mong đợi 42, nhận %d", entry.RevisionID)
	}
	if entry.TenantID != "tenant-123" {
		t.Errorf("TenantID không khớp: %s", entry.TenantID)
	}
	if entry.Subject != "user:alice" || entry.Action != "READ" || entry.Resource != "file:secret.pdf" {
		t.Errorf("Dữ liệu Subject/Action/Resource không khớp: %+v", entry)
	}
	if entry.Decision != "ALLOW" {
		t.Errorf("Decision không khớp: %s", entry.Decision)
	}
	if entry.MatchedPolicyID != "P-ALLOW-1" {
		t.Errorf("MatchedPolicyID không khớp: %s", entry.MatchedPolicyID)
	}
	if entry.Context["ip"] != "192.168.1.100" || entry.Context["device"] != "ios_secure" {
		t.Errorf("Context map không khớp: %+v", entry.Context)
	}
	if entry.Timestamp <= 0 {
		t.Errorf("Timestamp không hợp lệ: %d", entry.Timestamp)
	}
}

func TestAuditLogger_SpecialCharactersEscape(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewStreamAuditLogger(buf)
	defer logger.Stop()

	ctxMap := map[string]string{
		"user_agent": "Mozilla/5.0 \"quoted\"\nline2\ttab\\slash",
	}

	logger.Log(10, "tenant\"bad", "user:bob\nadmin", "WRITE", "doc\\secret", "DENY", "P-DENY", ctxMap)

	entry, err := DecodeNDJSONLogEntry(buf.Bytes())
	if err != nil {
		t.Fatalf("Không thể decode JSON có ký tự đặc biệt: %v (raw: %s)", err, buf.String())
	}

	if entry.TenantID != "tenant\"bad" {
		t.Errorf("TenantID escape không đúng: %s", entry.TenantID)
	}
	if entry.Subject != "user:bob\nadmin" {
		t.Errorf("Subject escape không đúng: %s", entry.Subject)
	}
	if entry.Context["user_agent"] != "Mozilla/5.0 \"quoted\"\nline2\ttab\\slash" {
		t.Errorf("Context escape không đúng: %s", entry.Context["user_agent"])
	}
}

func TestAuditLogger_ConcurrentZeroBlock(t *testing.T) {
	logger := NewStreamAuditLogger(&noopWriter{})
	defer logger.Stop()

	var wg sync.WaitGroup
	// 50 goroutines ghi đồng thời
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger.Log(uint64(id), "tenant-stress", "user:bob", "WRITE", "doc:data", "DENY", "P-DENY", map[string]string{
					"key": "val",
				})
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkAuditLogger_LogZeroAlloc(b *testing.B) {
	logger := NewStreamAuditLogger(&noopWriter{})
	defer logger.Stop()

	ctxMap := map[string]string{
		"ip": "10.0.0.1",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Log(1, "tenant-bench", "user:alice", "READ", "resource:doc", "ALLOW", "P-1", ctxMap)
	}
}
