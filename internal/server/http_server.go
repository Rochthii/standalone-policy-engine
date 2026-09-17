package server

import (
	"fmt"
	"net"
	"net/http"

	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/storage"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPServer cung cấp REST API cho Control Plane (CRUD chính sách) và Data Plane Fallback.
type HTTPServer struct {
	storage      *storage.Storage
	engine       *engine.EngineWithGC
	syncer       *engine.Syncer
	jwtValidator *security.JWTValidator
}

// NewHTTPServer khởi tạo mới một instance HTTPServer.
func NewHTTPServer(store *storage.Storage, eng *engine.EngineWithGC) *HTTPServer {
	return &HTTPServer{
		storage:      store,
		engine:       eng,
		syncer:       nil,
		jwtValidator: security.NewJWTValidator(),
	}
}

func NewHTTPServerWithSecurity(store *storage.Storage, eng *engine.EngineWithGC, securityConfig config.SecurityConfig) *HTTPServer {
	return &HTTPServer{
		storage: store,
		engine:  eng,
		syncer:  nil,
		jwtValidator: security.NewJWTValidatorWithConfig(
			securityConfig.JWTSecret,
			securityConfig.JWTIssuer,
			securityConfig.JWTAudience,
		),
	}
}

func NewHTTPServerWithSecurityAndSync(store *storage.Storage, eng *engine.EngineWithGC, syncer *engine.Syncer, securityConfig config.SecurityConfig) *HTTPServer {
	s := NewHTTPServerWithSecurity(store, eng, securityConfig)
	s.syncer = syncer
	return s
}

// ConfigureMux cấu hình router sử dụng ServeMux tiêu chuẩn Go 1.22+.
// Các endpoint Control Plane được bảo vệ bởi TenantAuthMiddleware (JWT + cross-tenant check).
func (s *HTTPServer) ConfigureMux() *http.ServeMux {
	mux := http.NewServeMux()
	registerHealthEndpoints(mux, s.storage, s.syncer)
	tenantAuth := TenantAuthMiddleware(s.jwtValidator)
	protect := func(permission string, handler http.Handler) http.Handler {
		return tenantAuth(RequirePermission(permission)(handler))
	}

	// Control Plane API endpoints require tenant isolation and explicit permission.
	mux.Handle("POST /api/v1/tenants/{tenant_id}/policies",
		protect("policy:write", http.HandlerFunc(s.handleCreatePolicy)))
	mux.Handle("PUT /api/v1/tenants/{tenant_id}/policies/{policy_id}",
		protect("policy:write", http.HandlerFunc(s.handleUpdatePolicy)))
	mux.Handle("DELETE /api/v1/tenants/{tenant_id}/policies/{policy_id}",
		protect("policy:write", http.HandlerFunc(s.handleDeletePolicy)))
	mux.Handle("POST /api/v1/tenants/{tenant_id}/policies/{policy_id}/publish",
		protect("policy:write", http.HandlerFunc(s.handlePublishPolicy)))
	mux.Handle("POST /api/v1/tenants/{tenant_id}/simulate",
		protect("policy:simulate", http.HandlerFunc(s.handleSimulate)))
	mux.Handle("GET /api/v1/tenants/{tenant_id}/schema",
		protect("policy:read", http.HandlerFunc(s.handleGetTenantSchema)))
	mux.Handle("POST /api/v1/tenants/{tenant_id}/prewarm",
		protect("policy:operate", http.HandlerFunc(s.handlePrewarm)))

	// Data Plane Fallback REST endpoints use the same authenticated identity boundary as gRPC.
	mux.Handle("POST /api/v1/decisions", tenantAuth(http.HandlerFunc(s.handleDecisions)))
	mux.Handle("POST /api/v1/decisions/explain", tenantAuth(http.HandlerFunc(s.handleExplain)))

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())

	return mux
}

// StartHTTPServer khởi chạy HTTP server tại cổng chỉ định.
func StartHTTPServer(port int, store *storage.Storage, eng *engine.EngineWithGC, securityConfig config.SecurityConfig) (*http.Server, error) {
	s := NewHTTPServerWithSecurity(store, eng, securityConfig)
	return startHTTPServer(port, s)
}

func StartHTTPServerWithSync(port int, store *storage.Storage, eng *engine.EngineWithGC, syncer *engine.Syncer, securityConfig config.SecurityConfig) (*http.Server, error) {
	s := NewHTTPServerWithSecurityAndSync(store, eng, syncer, securityConfig)
	return startHTTPServer(port, s)
}

func startHTTPServer(port int, s *HTTPServer) (*http.Server, error) {
	mux := s.ConfigureMux()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}

	go func() {
		_ = server.Serve(listener)
	}()

	return server, nil
}
