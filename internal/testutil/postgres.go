package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// CreateIsolatedPostgresDatabase returns a test-owned database URL and removes
// that database after the calling test completes.
func CreateIsolatedPostgresDatabase(t testing.TB, ctx context.Context, adminConnStr string) string {
	t.Helper()

	adminURL, err := url.Parse(adminConnStr)
	if err != nil || (adminURL.Scheme != "postgres" && adminURL.Scheme != "postgresql") {
		t.Fatalf("TEST_DATABASE_URL must be a valid PostgreSQL URL: %v", err)
	}
	adminConn, err := pgx.Connect(ctx, adminConnStr)
	if err != nil {
		t.Fatalf("connect PostgreSQL test admin: %v", err)
	}

	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		adminConn.Close(ctx)
		t.Fatalf("generate random database name: %v", err)
	}
	testDBName := "policy_engine_test_" + hex.EncodeToString(randomBytes)
	quotedDBName := pgx.Identifier{testDBName}.Sanitize()
	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+quotedDBName); err != nil {
		adminConn.Close(ctx)
		t.Fatalf("create isolated database %s: %v", testDBName, err)
	}

	t.Cleanup(func() {
		defer adminConn.Close(ctx)
		if !strings.HasPrefix(testDBName, "policy_engine_test_") {
			t.Errorf("refusing to drop database outside test namespace: %s", testDBName)
			return
		}
		if _, err := adminConn.Exec(ctx, "DROP DATABASE "+quotedDBName+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated database %s: %v", testDBName, err)
		}
	})

	testURL := *adminURL
	testURL.Path = "/" + testDBName
	return testURL.String()
}
