package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"standalone-policy-engine/internal/security"
)

const spillFormatVersion = 1

var ErrSpillQuota = errors.New("audit spill quota exceeded")

type spillBatch struct {
	Version int         `json:"version"`
	BatchID string      `json:"batch_id"`
	Entries []*LogEntry `json:"entries"`
}

type SpillStore struct {
	dir      string
	maxBytes int64
	crypto   *security.EnvelopeCrypto
	mu       sync.Mutex
}

func NewSpillStore(dir string, maxBytes int64, crypto *security.EnvelopeCrypto) (*SpillStore, error) {
	if strings.TrimSpace(dir) == "" || maxBytes <= 0 || crypto == nil {
		return nil, errors.New("audit spill directory, positive quota and crypto are required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create audit spill directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("protect audit spill directory: %w", err)
	}
	return &SpillStore{dir: dir, maxBytes: maxBytes, crypto: crypto}, nil
}

func (s *SpillStore) WriteBatch(entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		if _, err := VerifyAndDecryptEntry(s.crypto, entry); err != nil {
			return fmt.Errorf("refuse invalid audit spill entry: %w", err)
		}
	}
	batchID := uuid.NewString()
	data, err := json.Marshal(spillBatch{Version: spillFormatVersion, BatchID: batchID, Entries: entries})
	if err != nil {
		return err
	}
	used, err := s.sizeLocked()
	if err != nil {
		return err
	}
	if used+int64(len(data)) > s.maxBytes {
		return ErrSpillQuota
	}

	base := fmt.Sprintf("%020d-%s.auditspill", time.Now().UnixNano(), batchID)
	finalPath := filepath.Join(s.dir, base)
	tempPath := finalPath + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	removeTemp := true
	defer func() {
		_ = file.Close()
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func (s *SpillStore) Replay(ctx context.Context, writer BatchWriter) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}
	var replayed uint64
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".auditspill") {
			continue
		}
		path := filepath.Join(s.dir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return replayed, err
		}
		var batch spillBatch
		if err := json.Unmarshal(data, &batch); err != nil || batch.Version != spillFormatVersion || batch.BatchID == "" {
			return replayed, fmt.Errorf("invalid audit spill file %s", file.Name())
		}
		for _, entry := range batch.Entries {
			if _, err := VerifyAndDecryptEntry(s.crypto, entry); err != nil {
				return replayed, fmt.Errorf("tampered audit spill file %s: %w", file.Name(), err)
			}
		}
		if err := writer.InsertAuditLogsBatch(ctx, batch.Entries); err != nil {
			return replayed, err
		}
		if err := os.Remove(path); err != nil {
			return replayed, err
		}
		replayed += uint64(len(batch.Entries))
	}
	return replayed, nil
}

func (s *SpillStore) sizeLocked() (int64, error) {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".auditspill") {
			continue
		}
		info, err := file.Info()
		if err != nil {
			return 0, err
		}
		total += info.Size()
	}
	return total, nil
}
