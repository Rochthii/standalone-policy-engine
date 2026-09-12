// Package server cung cấp các middleware dùng chung cho cả HTTP và gRPC handlers.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"standalone-policy-engine/internal/security"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey là kiểu riêng tránh va chạm với các key khác trong context.
type contextKey string

const (
	// ContextKeyTenantID lưu tenant_id đã xác thực trong HTTP request context.
	ContextKeyTenantID contextKey = "authenticated_tenant_id"
	contextKeyIdentity contextKey = "authenticated_identity"
)

type authenticatedIdentity struct {
	TenantID            string
	Subject             string
	PrincipalAttributes map[string]string
	Claims              jwt.MapClaims
}

// TenantAuthMiddleware là HTTP middleware xác thực JWT và ràng buộc tenant_id:
//  1. Đọc JWT từ header Authorization (Bearer <token>).
//  2. Validate chữ ký và thời hạn token.
//  3. Trích xuất tenant_id từ claims.
//  4. Nếu path chứa {tenant_id}, kiểm tra giá trị khớp với claim.
//  5. Lưu tenant_id đã xác thực vào request context để handler dùng.
func TenantAuthMiddleware(jwtValidator *security.JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if jwtValidator == nil {
				http.Error(w, `{"error":"JWT validator is not configured"}`, http.StatusInternalServerError)
				return
			}
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.Fields(authHeader)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				http.Error(w, `{"error":"Authorization must use Bearer token"}`, http.StatusUnauthorized)
				return
			}

			claims, err := jwtValidator.ValidateToken(parts[1])
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

			subject, attributes, err := jwtValidator.ExtractSubjectAttributes(claims)
			if err != nil {
				http.Error(w, `{"error":"JWT missing valid sub claim"}`, http.StatusUnauthorized)
				return
			}

			identity := authenticatedIdentity{
				TenantID:            tenantIDFromToken,
				Subject:             subject,
				PrincipalAttributes: attributes,
				Claims:              claims,
			}

			// Ghi identity đã xác thực vào context để handler chỉ dùng dữ liệu tin cậy.
			ctx := context.WithValue(r.Context(), ContextKeyTenantID, tenantIDFromToken)
			ctx = context.WithValue(ctx, contextKeyIdentity, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission authorizes an already-authenticated HTTP identity using a
// signed JWT permissions claim.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := identityFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"authenticated identity is missing"}`, http.StatusUnauthorized)
				return
			}
			if !security.HasPermission(identity.Claims, permission) {
				http.Error(w, `{"error":"insufficient permission"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
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

func identityFromContext(ctx context.Context) (authenticatedIdentity, bool) {
	identity, ok := ctx.Value(contextKeyIdentity).(authenticatedIdentity)
	return identity, ok
}

// mergeTrustedPrincipalContext removes caller-supplied identity attributes and
// repopulates the principal namespace exclusively from signed JWT claims.
func mergeTrustedPrincipalContext(requestContext, attributes map[string]string) map[string]string {
	trustedContext := make(map[string]string, len(requestContext)+len(attributes))
	for key, value := range requestContext {
		if key == "tenant_id" || strings.HasPrefix(key, "principal.") {
			continue
		}
		trustedContext[key] = value
	}

	for key, value := range attributes {
		if key == "tenant_id" {
			continue
		}
		switch {
		case strings.HasPrefix(key, "principal."):
			trustedContext[key] = value
		case !strings.Contains(key, "."):
			trustedContext["principal."+key] = value
		}
	}

	return trustedContext
}
