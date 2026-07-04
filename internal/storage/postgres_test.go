package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestStorage_MigrationsIntegration(t *testing.T) {
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	// Ket noi toi database mac dinh postgres de tao/xoa database kiem thu
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Skipf("Bo qua test tich hop Postgres: khong the ket noi toi %s: %v", connStr, err)
	}
	defer conn.Close(ctx)

	// Ten database kiem thu tam thoi
	testDBName := "policy_engine_migration_test"
	
	// Xoa neu da ton tai
	_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+testDBName)
	
	// Tao database moi
	_, err = conn.Exec(ctx, "CREATE DATABASE "+testDBName)
	if err != nil {
		t.Fatalf("Khong the tao database kiem thu %s: %v", testDBName, err)
	}
	defer func() {
		// Don dep: Xoa database sau khi ket thuc test
		_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+testDBName)
	}()

	// Chuoi ket noi toi database kiem thu tam thoi
	testDBConnStr := "postgres://postgres:postgres@localhost:5432/" + testDBName + "?sslmode=disable"

	// Khoi tao Storage - qua trinh nay se tu dong chay runMigrations
	store, err := NewStorage(testDBConnStr)
	if err != nil {
		t.Fatalf("Khoi tao Storage va chay migration that bai: %v", err)
	}
	defer store.Close()

	// Kiem tra cac bang da duoc tao thanh cong chua
	var exists bool
	err = store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'policies'
		);
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Kiem tra bang 'policies' gap loi: %v", err)
	}
	if !exists {
		t.Error("Bang 'policies' khong duoc tao boi migrations")
	}

	err = store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'audit_logs'
		);
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Kiem tra bang 'audit_logs' gap loi: %v", err)
	}
	if !exists {
		t.Error("Bang 'audit_logs' khong duoc tao boi migrations")
	}
}
