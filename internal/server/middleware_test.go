package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"standalone-policy-engine/internal/security"
)

// makeTestToken tạo JWT token với các claims cho mục đích test.
func makeTestToken(tenantID, subject string, exp time.Duration) string {
	claims := jwt.MapClaims{
		"sub":       subject,
		"tenant_id": tenantID,
		"exp":       time.Now().Add(exp).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("test-secret-key-for-sprint-6-unit-test"))
	return tokenStr
}

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-sprint-6-unit-test")
	defer os.Unsetenv("JWT_SECRET")
	os.Exit(m.Run())
}

// TestTenantAuthMiddlewareMissingAuthHeader kiem thu tu choi request khong co Authorization header.
func TestTenantAuthMiddlewareMissingAuthHeader(t *testing.T) {
	jv := security.NewJWTValidator()
	handler := TenantAuthMiddleware(jv)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-a/policies", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Mong 401 Unauthorized, thuc te %d", rr.Code)
	}
}

// TestTenantAuthMiddlewareInvalidToken kiem thu tu choi token co chu ky sai.
func TestTenantAuthMiddlewareInvalidToken(t *testing.T) {
	jv := security.NewJWTValidator()
	handler := TenantAuthMiddleware(jv)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-a/policies", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.string")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Mong 401 Unauthorized voi token sai, thuc te %d", rr.Code)
	}
}

// TestTenantAuthMiddlewareCrossTenantAttack kiem thu chong cross-tenant attack:
// token cua tenant-A khong duoc phep tac dong len resource cua tenant-B.
func TestTenantAuthMiddlewareCrossTenantAttack(t *testing.T) {
	jv := security.NewJWTValidator()

	var capturedTenantID string
	handler := TenantAuthMiddleware(jv)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Token thuoc tenant-A nhung request toi endpoint cua tenant-B
	tokenStr := makeTestToken("tenant-a", "user:attacker", 1*time.Hour)

	// Simulate path param: tenant_id = tenant-b
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-b/policies", nil)
	req.SetPathValue("tenant_id", "tenant-b") // Go 1.22+ PathValue
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Middleware phai tra 403 Forbidden vi tenant_id trong token != tenant_id trong path
	if rr.Code != http.StatusForbidden {
		t.Errorf("Mong 403 Forbidden cho cross-tenant attack, thuc te %d (capturedTenant=%s)", rr.Code, capturedTenantID)
	}
}

// TestTenantAuthMiddlewareValidSameTenant kiem thu request hop le (token khop voi path tenant).
func TestTenantAuthMiddlewareValidSameTenant(t *testing.T) {
	jv := security.NewJWTValidator()

	var capturedTenantID string
	handler := TenantAuthMiddleware(jv)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := makeTestToken("tenant-a", "user:alice", 1*time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-a/policies", nil)
	req.SetPathValue("tenant_id", "tenant-a")
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Mong 200 OK voi token hop le, thuc te %d", rr.Code)
	}
	if capturedTenantID != "tenant-a" {
		t.Errorf("Captured tenant_id sai: mong tenant-a, thuc te %s", capturedTenantID)
	}
}

// TestTenantAuthMiddlewareExpiredToken kiem thu tu choi token het han.
func TestTenantAuthMiddlewareExpiredToken(t *testing.T) {
	jv := security.NewJWTValidator()

	handler := TenantAuthMiddleware(jv)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Token da het han 1 gio truoc
	tokenStr := makeTestToken("tenant-a", "user:alice", -1*time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-a/policies", nil)
	req.SetPathValue("tenant_id", "tenant-a")
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Mong 401 Unauthorized voi token het han, thuc te %d", rr.Code)
	}
}

// TestTenantAuthMiddlewareMissingTenantIDClaim kiem thu tu choi token thieu tenant_id claim.
func TestTenantAuthMiddlewareMissingTenantIDClaim(t *testing.T) {
	jv := security.NewJWTValidator()

	handler := TenantAuthMiddleware(jv)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Token khong co tenant_id claim
	claims := jwt.MapClaims{
		"sub": "user:alice",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("test-secret-key-for-sprint-6-unit-test"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-a/policies", nil)
	req.SetPathValue("tenant_id", "tenant-a")
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Mong 403 Forbidden khi thieu tenant_id claim, thuc te %d", rr.Code)
	}
}

// TestTenantIDFromContextEmpty kiem thu tra ve chuoi rong khi context khong co tenant_id.
func TestTenantIDFromContextEmpty(t *testing.T) {
	result := TenantIDFromContext(context.Background())
	if result != "" {
		t.Errorf("Mong chuoi rong, thuc te %q", result)
	}
}
