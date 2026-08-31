package storage

import (
	"context"
	"encoding/json"
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
	pool *pgxpool.Pool
}

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

	s := &Storage{pool: pool}

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
func (s *Storage) UpdatePolicy(ctx context.Context, policyID, policyText string) error {
	query := `UPDATE policies SET policy_text = $1, status = 'DRAFT', updated_at = CURRENT_TIMESTAMP WHERE id = $2;`
	tag, err := s.pool.Exec(ctx, query, policyText, policyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy policy để cập nhật: %s", policyID)
	}
	return nil
}

// DBPolicyUpdateEvent cấu trúc tin nhắn thông báo cập nhật qua PostgreSQL NOTIFY.
type DBPolicyUpdateEvent struct {
	TenantID string `json:"tenant_id"`
	PolicyID string `json:"policy_id"`
	Action   string `json:"action"`
	Revision uint64 `json:"revision"`
}

// PublishPolicy xuất bản một chính sách: đổi status sang ACTIVE, lưu AST JSON, tăng version và tăng revision của Tenant nguyên tử trong Transaction.
func (s *Storage) PublishPolicy(ctx context.Context, policyID string, astJSON []byte) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var version int
	var tenantID string
	query := `UPDATE policies 
              SET status = 'ACTIVE', ast_json = $1, version = version + 1, updated_at = CURRENT_TIMESTAMP 
              WHERE id = $2 RETURNING version, tenant_id;`
	err = tx.QueryRow(ctx, query, astJSON, policyID).Scan(&version, &tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("không tìm thấy policy để publish: %s", policyID)
		}
		return 0, err
	}

	var newRevision uint64
	revQuery := `UPDATE tenants 
                 SET revision = revision + 1, updated_at = CURRENT_TIMESTAMP 
                 WHERE id = $1 RETURNING revision;`
	err = tx.QueryRow(ctx, revQuery, tenantID).Scan(&newRevision)
	if err != nil {
		// Fallback nếu bảng tenants chưa có dòng hoặc lỗi
		newRevision = 1
	}

	// Phát thông báo pg_notify (chỉ được gửi khi Transaction commit thành công)
	notifyQuery := fmt.Sprintf(`NOTIFY policy_events, '{"tenant_id":"%s","policy_id":"%s","action":"UPDATE","revision":%d}';`, tenantID, policyID, newRevision)
	if _, err := tx.Exec(ctx, notifyQuery); err != nil {
		log.Printf("[Storage] Cảnh báo lỗi NOTIFY: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return version, nil
}

// GetPolicy lấy thông tin chi tiết của một chính sách.
func (s *Storage) GetPolicy(ctx context.Context, policyID string) (*DBPolicy, error) {
	query := `SELECT id, tenant_id, effect, policy_text, ast_json, version, status, created_at, updated_at 
              FROM policies WHERE id = $1;`
	row := s.pool.QueryRow(ctx, query, policyID)
	
	p := &DBPolicy{}
	err := row.Scan(&p.ID, &p.TenantID, &p.Effect, &p.PolicyText, &p.ASTJSON, &p.Version, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePolicy xóa bỏ một chính sách và tăng revision của Tenant nguyên tử trong Transaction.
func (s *Storage) DeletePolicy(ctx context.Context, policyID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var tenantID string
	query := `DELETE FROM policies WHERE id = $1 RETURNING tenant_id;`
	err = tx.QueryRow(ctx, query, policyID).Scan(&tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("không tìm thấy policy để xóa: %s", policyID)
		}
		return err
	}

	var newRevision uint64
	revQuery := `UPDATE tenants 
                 SET revision = revision + 1, updated_at = CURRENT_TIMESTAMP 
                 WHERE id = $1 RETURNING revision;`
	err = tx.QueryRow(ctx, revQuery, tenantID).Scan(&newRevision)
	if err != nil {
		newRevision = 1
	}

	notifyQuery := fmt.Sprintf(`NOTIFY policy_events, '{"tenant_id":"%s","policy_id":"%s","action":"DELETE","revision":%d}';`, tenantID, policyID, newRevision)
	if _, err := tx.Exec(ctx, notifyQuery); err != nil {
		log.Printf("[Storage] Cảnh báo lỗi NOTIFY: %v", err)
	}

	return tx.Commit(ctx)
}

// GetTenantRevision lấy số hiệu phiên bản revision hiện tại của một Tenant từ PostgreSQL.
func (s *Storage) GetTenantRevision(ctx context.Context, tenantID string) (uint64, error) {
	var revision uint64
	query := `SELECT COALESCE(revision, 1) FROM tenants WHERE id = $1;`
	err := s.pool.QueryRow(ctx, query, tenantID).Scan(&revision)
	if err != nil {
		return 1, nil // Fallback mặc định
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

	// Sử dụng CopyFrom của pgx để đạt hiệu năng ghi đĩa tối đa
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

		entries[i] = []interface{}{
			l.TenantID,
			subjectVal,
			actionVal,
			resourceVal,
			l.Decision,
			l.MatchedPolicyID,
			contextVal,
			l.IsEncrypted,
			l.EncryptedDEK,
			l.EncryptedPayload,
		}
	}

	_, err := s.pool.CopyFrom(
		ctx,
		pgx.Identifier{"decision_audit_logs"},
		[]string{
			"tenant_id",
			"request_subject",
			"request_action",
			"request_resource",
			"decision",
			"matched_policy_id",
			"evaluated_context",
			"is_encrypted",
			"encrypted_dek",
			"encrypted_payload",
		},
		pgx.CopyFromRows(entries),
	)
	return err
}
