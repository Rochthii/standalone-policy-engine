package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// Tự động khởi tạo schema cơ sở dữ liệu
	if err := s.initSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("khởi tạo schema thất bại: %v", err)
	}

	return s, nil
}

// Close đóng các kết nối trong pool.
func (s *Storage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Storage) initSchema(ctx context.Context) error {
	// Kịch bản schema thực tế dựa trên docs/data/data-model.md
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
		
		`CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL UNIQUE,
			status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS policies (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			effect VARCHAR(10) NOT NULL,
			policy_text TEXT NOT NULL,
			ast_json JSONB,
			version INT NOT NULL DEFAULT 1,
			status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_policies_tenant_status ON policies(tenant_id, status);`,

		`CREATE TABLE IF NOT EXISTS decision_audit_logs (
			id BIGSERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			request_subject VARCHAR(255) NOT NULL,
			request_action VARCHAR(255) NOT NULL,
			request_resource VARCHAR(255) NOT NULL,
			decision VARCHAR(10) NOT NULL,
			matched_policy_id UUID,
			evaluated_context JSONB,
			evaluated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_time ON decision_audit_logs(tenant_id, evaluated_at DESC);`,
	}

	// Chạy từng câu lệnh tuần tự
	for _, q := range queries {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("lỗi thực thi query: %s. Chi tiết: %v", q, err)
		}
	}

	return nil
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

// PublishPolicy xuất bản một chính sách: đổi status sang ACTIVE, lưu AST JSON và tăng version.
func (s *Storage) PublishPolicy(ctx context.Context, policyID string, astJSON []byte) (int, error) {
	var version int
	query := `UPDATE policies 
              SET status = 'ACTIVE', ast_json = $1, version = version + 1, updated_at = CURRENT_TIMESTAMP 
              WHERE id = $2 RETURNING version;`
	err := s.pool.QueryRow(ctx, query, astJSON, policyID).Scan(&version)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("không tìm thấy policy để publish: %s", policyID)
		}
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

// DeletePolicy xóa bỏ một chính sách khỏi DB.
func (s *Storage) DeletePolicy(ctx context.Context, policyID string) error {
	query := `DELETE FROM policies WHERE id = $1;`
	tag, err := s.pool.Exec(ctx, query, policyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy policy để xóa: %s", policyID)
	}
	return nil
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
		entries[i] = []interface{}{
			l.TenantID,
			l.Subject,
			l.Action,
			l.Resource,
			l.Decision,
			l.MatchedPolicyID,
			l.Context, // Chứa chuỗi JSON hoặc map
		}
	}

	_, err := s.pool.CopyFrom(
		ctx,
		pgx.Identifier{"decision_audit_logs"},
		[]string{"tenant_id", "request_subject", "request_action", "request_resource", "decision", "matched_policy_id", "evaluated_context"},
		pgx.CopyFromRows(entries),
	)
	return err
}
