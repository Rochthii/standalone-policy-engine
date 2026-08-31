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

func TestAuditLogger_BinaryPackagingAndDecoding(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewStreamAuditLogger(buf)
	defer logger.Stop()

	ctxMap := map[string]string{
		"ip":     "192.168.1.100",
		"device": "ios_secure",
	}

	logger.Log("tenant-123", "user:alice", "READ", "file:secret.pdf", "ALLOW", "P-ALLOW-1", ctxMap)

	if buf.Len() == 0 {
		t.Fatal("Buffer không được rỗng sau khi ghi log")
	}

	entry, err := DecodeBinaryLogEntry(buf.Bytes())
	if err != nil {
		t.Fatalf("Decode binary log entry lỗi: %v", err)
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
}

func TestAuditLogger_ConcurrentZeroBlock(t *testing.T) {
	logger := NewStreamAuditLogger(&noopWriter{})
	defer logger.Stop()

	var wg sync.WaitGroup
	// 50 goroutines ghi đồng thời
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger.Log("tenant-stress", "user:bob", "WRITE", "doc:data", "DENY", "P-DENY", map[string]string{
					"key": "val",
				})
			}
		}()
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
		logger.Log("tenant-bench", "user:alice", "READ", "resource:doc", "ALLOW", "P-1", ctxMap)
	}
}
