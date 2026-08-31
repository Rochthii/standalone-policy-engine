package server

import (
	"fmt"
	"net"
	"net/http"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/storage"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPServer cung cấp REST API cho Control Plane (CRUD chính sách) và Data Plane Fallback.
type HTTPServer struct {
	storage *storage.Storage
	engine  *engine.EngineWithGC
}

// NewHTTPServer khởi tạo mới một instance HTTPServer.
func NewHTTPServer(store *storage.Storage, eng *engine.EngineWithGC) *HTTPServer {
	return &HTTPServer{
		storage: store,
		engine:  eng,
	}
}

// ConfigureMux cấu hình router sử dụng ServeMux tiêu chuẩn Go 1.22+.
// Các endpoint Control Plane được bảo vệ bởi TenantAuthMiddleware (JWT + cross-tenant check).
func (s *HTTPServer) ConfigureMux() *http.ServeMux {
	mux := http.NewServeMux()
	jwtValidator := security.NewJWTValidator()
	tenantAuth := TenantAuthMiddleware(jwtValidator)

	// Control Plane API endpoints — yêu cầu JWT hợp lệ và tenant isolation
	mux.Handle("POST /api/v1/tenants/{tenant_id}/policies",
		tenantAuth(http.HandlerFunc(s.handleCreatePolicy)))
	mux.Handle("PUT /api/v1/tenants/{tenant_id}/policies/{policy_id}",
		tenantAuth(http.HandlerFunc(s.handleUpdatePolicy)))
	mux.Handle("DELETE /api/v1/tenants/{tenant_id}/policies/{policy_id}",
		tenantAuth(http.HandlerFunc(s.handleDeletePolicy)))
	mux.Handle("POST /api/v1/tenants/{tenant_id}/policies/{policy_id}/publish",
		tenantAuth(http.HandlerFunc(s.handlePublishPolicy)))
	mux.Handle("POST /api/v1/tenants/{tenant_id}/simulate",
		tenantAuth(http.HandlerFunc(s.handleSimulate)))
	mux.Handle("GET /api/v1/tenants/{tenant_id}/schema",
		tenantAuth(http.HandlerFunc(s.handleGetTenantSchema)))
	mux.Handle("POST /api/v1/tenants/{tenant_id}/prewarm",
		tenantAuth(http.HandlerFunc(s.handlePrewarm)))

	// Data Plane Fallback REST endpoints — không yêu cầu tenant auth (PDP public)
	mux.HandleFunc("POST /api/v1/decisions", s.handleDecisions)
	mux.HandleFunc("POST /api/v1/decisions/explain", s.handleExplain)

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())

	return mux
}

// StartHTTPServer khởi chạy HTTP server tại cổng chỉ định.
func StartHTTPServer(port int, store *storage.Storage, eng *engine.EngineWithGC) (*http.Server, error) {
	s := NewHTTPServer(store, eng)
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
