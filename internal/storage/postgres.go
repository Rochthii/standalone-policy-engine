package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"standalone-policy-engine/internal/audit"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// DBPolicy đại diện cho một dòng trong bảng policies của PostgreSQL.
type DBPolicy struct {
	ID         string
	TenantID   string
	Effect     string
	PolicyText string
	ASTJSON    []byte
	Version    int
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Storage quản lý các kết nối và truy vấn đến PostgreSQL database.
type Storage struct {
	pool           *pgxpool.Pool
	policyNotifier policyNotifier
}

type policyNotifier func(context.Context, pgx.Tx, string) error

var ErrPolicyNotFound = errors.New("policy not found")

// NewStorage khởi tạo kết nối database và chạy script DDL tự động khởi tạo bảng.
func NewStorage(connStr string) (*Storage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("phân tích connection string thất bại: %v", err)
	}

	// Cấu hình pool tối ưu hiệu năng
	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("khởi tạo connection pool thất bại: %v", err)
	}

	s := &Storage{pool: pool, policyNotifier: postgresPolicyNotifier}

	// Tự động khởi tạo schema cơ sở dữ liệu qua golang-migrate
	if err := s.runMigrations(connStr); err != nil {
		pool.Close()
		return nil, fmt.Errorf("chạy migration thất bại: %v", err)
	}

	return s, nil
}

// Close đóng các kết nối trong pool.
func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Storage) runMigrations(connStr string) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}

	sourceURL := fmt.Sprintf("file://%s", filepath.ToSlash(migrationsDir))
	m, err := migrate.New(sourceURL, connStr)
	if err != nil {
		return fmt.Errorf("khởi tạo migrate với source %s thất bại: %w", sourceURL, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("lỗi apply migrations: %w", err)
	}

	return nil
}

func locateMigrationsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Duyệt ngược lên tối đa 5 cấp thư mục để tìm db/migrations
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "db", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("không tìm thấy thư mục db/migrations bắt đầu từ %s", dir)
}

// CreateTenant tạo mới một Tenant và trả về UUID ID.
func (s *Storage) CreateTenant(ctx context.Context, name string) (string, error) {
	var id string
	query := `INSERT INTO tenants (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP RETURNING id;`
	err := s.pool.QueryRow(ctx, query, name).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// CreatePolicy tạo mới một chính sách ở trạng thái DRAFT.
func (s *Storage) CreatePolicy(ctx context.Context, tenantID, effect, policyText string) (string, error) {
	var id string
	query := `INSERT INTO policies (tenant_id, effect, policy_text, status, version) 
              VALUES ($1, $2, $3, 'DRAFT', 1) RETURNING id;`
	err := s.pool.QueryRow(ctx, query, tenantID, effect, policyText).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// UpdatePolicy cập nhật nội dung văn bản thô của một chính sách (reset về DRAFT).
func (s *Storage) UpdatePolicy(ctx context.Context, tenantID, policyID, policyText string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var previousStatus string
	query := `WITH target AS (
                SELECT status FROM policies WHERE id = $2 AND tenant_id = $3 FOR UPDATE
              )
              UPDATE policies AS p
              SET policy_text = $1, status = 'DRAFT', ast_json = NULL, updated_at = CURRENT_TIMESTAMP
              FROM target
              WHERE p.id = $2 AND p.tenant_id = $3
              RETURNING target.status;`
	if err := tx.QueryRow(ctx, query, policyText, policyID, tenantID).Scan(&previousStatus); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("%w: %s", ErrPolicyNotFound, policyID)
		}
		return err
	}
	if previousStatus == "ACTIVE" {
		if _, err := s.incrementTenantRevisionAndNotify(ctx, tx, tenantID, policyID, "UPDATE"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DBPolicyUpdateEvent cấu trúc tin nhắn thông báo cập nhật qua PostgreSQL NOTIFY.
type DBPolicyUpdateEvent struct {
	TenantID string `json:"tenant_id"`
	PolicyID string `json:"policy_id"`
	Action   string `json:"action"`
	Revision uint64 `json:"revision"`
}

// PublishPolicy xuất bản một chính sách: đổi status sang ACTIVE, lưu AST JSON, tăng version và tăng revision của Tenant nguyên tử trong Transaction.
func (s *Storage) PublishPolicy(ctx context.Context, tenantID, policyID string, astJSON []byte) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var version int
	query := `UPDATE policies 
              SET status = 'ACTIVE', ast_json = $1, version = version + 1, updated_at = CURRENT_TIMESTAMP 
              WHERE id = $2 AND tenant_id = $3 RETURNING version;`
	err = tx.QueryRow(ctx, query, astJSON, policyID, tenantID).Scan(&version)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("%w: %s", ErrPolicyNotFound, policyID)
		}
		return 0, err
	}

	if _, err := s.incrementTenantRevisionAndNotify(ctx, tx, tenantID, policyID, "UPDATE"); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return version, nil
}

// GetPolicy lấy thông tin chi tiết của một chính sách.
func (s *Storage) GetPolicy(ctx context.Context, tenantID, policyID string) (*DBPolicy, error) {
	query := `SELECT id, tenant_id, effect, policy_text, ast_json, version, status, created_at, updated_at 
              FROM policies WHERE id = $1 AND tenant_id = $2;`
	row := s.pool.QueryRow(ctx, query, policyID, tenantID)

	p := &DBPolicy{}
	err := row.Scan(&p.ID, &p.TenantID, &p.Effect, &p.PolicyText, &p.ASTJSON, &p.Version, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: %s", ErrPolicyNotFound, policyID)
		}
		return nil, err
	}
	return p, nil
}

// DeletePolicy xóa bỏ một chính sách và tăng revision của Tenant nguyên tử trong Transaction.
func (s *Storage) DeletePolicy(ctx context.Context, tenantID, policyID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `DELETE FROM policies WHERE id = $1 AND tenant_id = $2 RETURNING id;`
	var deletedPolicyID string
	err = tx.QueryRow(ctx, query, policyID, tenantID).Scan(&deletedPolicyID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("%w: %s", ErrPolicyNotFound, policyID)
		}
		return err
	}

	if _, err := s.incrementTenantRevisionAndNotify(ctx, tx, tenantID, policyID, "DELETE"); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) incrementTenantRevisionAndNotify(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, policyID, action string,
) (uint64, error) {
	var revision uint64
	revisionQuery := `UPDATE tenants
                      SET revision = revision + 1, updated_at = CURRENT_TIMESTAMP
                      WHERE id = $1 RETURNING revision;`
	if err := tx.QueryRow(ctx, revisionQuery, tenantID).Scan(&revision); err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("tenant not found while advancing policy revision: %s", tenantID)
		}
		return 0, fmt.Errorf("advance tenant revision: %w", err)
	}

	payload, err := json.Marshal(DBPolicyUpdateEvent{
		TenantID: tenantID,
		PolicyID: policyID,
		Action:   action,
		Revision: revision,
	})
	if err != nil {
		return 0, fmt.Errorf("encode policy event: %w", err)
	}
	notify := s.policyNotifier
	if notify == nil {
		notify = postgresPolicyNotifier
	}
	if err := notify(ctx, tx, string(payload)); err != nil {
		return 0, fmt.Errorf("notify policy event: %w", err)
	}
	return revision, nil
}

func postgresPolicyNotifier(ctx context.Context, tx pgx.Tx, payload string) error {
	_, err := tx.Exec(ctx, `SELECT pg_notify('policy_events', $1);`, payload)
	return err
}

// GetTenantRevision lấy số hiệu phiên bản revision hiện tại của một Tenant từ PostgreSQL.
func (s *Storage) GetTenantRevision(ctx context.Context, tenantID string) (uint64, error) {
	var revision uint64
	query := `SELECT COALESCE(revision, 1) FROM tenants WHERE id = $1;`
	err := s.pool.QueryRow(ctx, query, tenantID).Scan(&revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("tenant not found: %s", tenantID)
		}
		return 0, err
	}
	return revision, nil
}

// ListenPolicyEvents mở kết nối chuyên dụng để lắng nghe kênh 'policy_events' của PostgreSQL.
func (s *Storage) ListenPolicyEvents(ctx context.Context, callback func(event DBPolicyUpdateEvent)) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("lỗi acquire kết nối lắng nghe: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN policy_events;")
	if err != nil {
		return fmt.Errorf("lỗi thực thi LISTEN policy_events: %w", err)
	}

	log.Println("[Storage] Đang lắng nghe kênh PostgreSQL 'policy_events'...")

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("lỗi WaitForNotification: %w", err)
		}

		var ev DBPolicyUpdateEvent
		if err := json.Unmarshal([]byte(notification.Payload), &ev); err == nil {
			callback(ev)
		}
	}
}

// GetActivePolicies lấy danh sách tất cả các chính sách đang hoạt động (ACTIVE) của một Tenant.
func (s *Storage) GetActivePolicies(ctx context.Context, tenantID string) ([]*DBPolicy, error) {
	query := `SELECT id, tenant_id, effect, policy_text, ast_json, version, status, created_at, updated_at 
              FROM policies WHERE tenant_id = $1 AND status = 'ACTIVE';`
	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make([]*DBPolicy, 0)
	for rows.Next() {
		p := &DBPolicy{}
		err := rows.Scan(&p.ID, &p.TenantID, &p.Effect, &p.PolicyText, &p.ASTJSON, &p.Version, &p.Status, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}

	return policies, nil
}

// InsertAuditLogsBatch ghi danh sách logs kiểm toán vào database (Batch Insert).
func (s *Storage) InsertAuditLogsBatch(ctx context.Context, logs []*audit.LogEntry) error {
	if len(logs) == 0 {
		return nil
	}

	// Copy into a transaction-local staging table, then merge idempotently by audit_id.
	entries := make([][]interface{}, len(logs))
	for i, l := range logs {
		var subjectVal, actionVal, resourceVal interface{}
		var contextVal interface{}

		if !l.IsEncrypted {
			subjectVal = l.Subject
			actionVal = l.Action
			resourceVal = l.Resource
			contextVal = l.Context
		}

		var matchedPolicyID interface{}
		if l.MatchedPolicyID != "" {
			matchedPolicyID = l.MatchedPolicyID
		}
		entries[i] = []interface{}{
			l.AuditID,
			l.Timestamp,
			l.TenantID,
			subjectVal,
			actionVal,
			resourceVal,
			l.Decision,
			matchedPolicyID,
			contextVal,
			l.IsEncrypted,
			int64(l.RevisionID),
			l.PayloadVersion,
			l.KeyID,
			l.RequestID,
			l.TraceID,
			l.EncryptedDEK,
			l.EncryptedPayload,
			l.IntegrityTag,
			time.Unix(0, l.Timestamp).UTC(),
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE audit_ingest ON COMMIT DROP AS
		SELECT audit_id, event_timestamp_ns, tenant_id, request_subject, request_action, request_resource,
			decision, matched_policy_id, evaluated_context, is_encrypted, revision_id,
			payload_version, key_id, request_id, trace_id, encrypted_dek,
			encrypted_payload, integrity_tag, evaluated_at
		FROM decision_audit_logs WITH NO DATA
	`); err != nil {
		return err
	}
	columns := []string{
		"audit_id", "event_timestamp_ns", "tenant_id", "request_subject", "request_action", "request_resource",
		"decision", "matched_policy_id", "evaluated_context", "is_encrypted", "revision_id",
		"payload_version", "key_id", "request_id", "trace_id", "encrypted_dek",
		"encrypted_payload", "integrity_tag", "evaluated_at",
	}
	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"audit_ingest"},
		columns,
		pgx.CopyFromRows(entries),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO decision_audit_logs (
			audit_id, event_timestamp_ns, tenant_id, request_subject, request_action, request_resource,
			decision, matched_policy_id, evaluated_context, is_encrypted, revision_id,
			payload_version, key_id, request_id, trace_id, encrypted_dek,
			encrypted_payload, integrity_tag, evaluated_at
		)
		SELECT audit_id, event_timestamp_ns, tenant_id, request_subject, request_action, request_resource,
			decision, matched_policy_id, evaluated_context, is_encrypted, revision_id,
			payload_version, key_id, request_id, trace_id, encrypted_dek,
			encrypted_payload, integrity_tag, evaluated_at
		FROM audit_ingest
		ON CONFLICT (audit_id) DO NOTHING
	`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
