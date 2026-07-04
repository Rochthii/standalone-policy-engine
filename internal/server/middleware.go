// Package server cung cấp các middleware dùng chung cho cả HTTP và gRPC handlers.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"standalone-policy-engine/internal/security"
)

// contextKey là kiểu riêng tránh va chạm với các key khác trong context.
type contextKey string

const (
	// ContextKeyTenantID lưu tenant_id đã xác thực trong HTTP request context.
	ContextKeyTenantID contextKey = "authenticated_tenant_id"
)

// TenantAuthMiddleware là HTTP middleware xác thực JWT và ràng buộc tenant_id:
//  1. Đọc JWT từ header Authorization (Bearer <token>).
//  2. Validate chữ ký và thời hạn token.
//  3. Trích xuất tenant_id từ claims.
//  4. Nếu path chứa {tenant_id}, kiểm tra giá trị khớp với claim.
//  5. Lưu tenant_id đã xác thực vào request context để handler dùng.
func TenantAuthMiddleware(jwtValidator *security.JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwtValidator.ValidateToken(tokenStr)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"invalid token: %v"}`, err), http.StatusUnauthorized)
				return
			}

			// Trích xuất tenant_id từ JWT claims
			tenantIDFromToken, ok := claims["tenant_id"].(string)
			if !ok || tenantIDFromToken == "" {
				http.Error(w, `{"error":"JWT missing tenant_id claim"}`, http.StatusForbidden)
				return
			}

			// Nếu request có path param tenant_id (ví dụ /api/v1/tenants/{tenant_id}/...),
			// kiểm tra khớp để chống cross-tenant attack.
			if pathTenantID := r.PathValue("tenant_id"); pathTenantID != "" {
				if pathTenantID != tenantIDFromToken {
					http.Error(w, `{"error":"tenant_id in path does not match token"}`, http.StatusForbidden)
					return
				}
			}

			// Ghi tenant_id đã xác thực vào context để handler có thể đọc an toàn
			ctx := context.WithValue(r.Context(), ContextKeyTenantID, tenantIDFromToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantIDFromContext lấy tenant_id đã xác thực từ HTTP request context.
// Trả về chuỗi rỗng nếu không tìm thấy.
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyTenantID).(string); ok {
		return v
	}
	return ""
}
