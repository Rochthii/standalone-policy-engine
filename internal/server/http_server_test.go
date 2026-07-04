package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/storage"
)

func TestHTTPServer_TenantIsolation(t *testing.T) {
	// Su dung cung secret key voi middleware_test.go
	secret := "test-secret-key-for-sprint-6-unit-test"

	store, _ := storage.NewStorage("postgres://postgres:postgres@localhost:5432/policy_engine_test?sslmode=disable")
	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled:     true,
		Interval:    1 * time.Hour,
		IdleTimeout: 1 * time.Hour,
	})
	srv := NewHTTPServer(store, eng, nil)
	mux := srv.ConfigureMux()

	makeToken := func(tenantID string) string {
		claims := jwt.MapClaims{
			"sub":       "user:admin",
			"tenant_id": tenantID,
			"exp":       time.Now().Add(1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte(secret))
		return tokenStr
	}

	t.Run("Create Policy - Mismatched Tenant ID", func(t *testing.T) {
		token := makeToken("tenant-a")
		reqBody := `{"effect":"permit","policy_text":"permit(principal == any, action == any, resource == any);"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-b/policies", strings.NewReader(reqBody))
		req.SetPathValue("tenant_id", "tenant-b") // Go 1.22+ Router path binding
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Mong doi 403 Forbidden cho cross-tenant request, thuc te %d", rr.Code)
		}
	})

	t.Run("Simulate - Matched Tenant ID but Invalid Token", func(t *testing.T) {
		reqBody := `{"subject":"user:alice","action":"READ","resource":"file:doc","policies":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-a/simulate", strings.NewReader(reqBody))
		req.SetPathValue("tenant_id", "tenant-a")
		req.Header.Set("Authorization", "Bearer invalid-token-string")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Mong doi 401 Unauthorized, thuc te %d", rr.Code)
		}
	})
}
