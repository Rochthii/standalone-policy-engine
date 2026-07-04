package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/storage"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// HTTPServer cung cấp REST API cho Control Plane (CRUD chính sách) và Data Plane Fallback.
type HTTPServer struct {
	storage     *storage.Storage
	engine      *engine.EngineWithGC
	redisClient redis.UniversalClient
}

// NewHTTPServer khởi tạo mới một instance HTTPServer.
func NewHTTPServer(store *storage.Storage, eng *engine.EngineWithGC, rdb redis.UniversalClient) *HTTPServer {
	return &HTTPServer{
		storage:     store,
		engine:      eng,
		redisClient: rdb,
	}
}

// ConfigureMux cấu hình router sử dụng ServeMux tiêu chuẩn Go 1.22+.
func (s *HTTPServer) ConfigureMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Control Plane API endpoints
	mux.HandleFunc("POST /api/v1/tenants/{tenant_id}/policies", s.handleCreatePolicy)
	mux.HandleFunc("PUT /api/v1/tenants/{tenant_id}/policies/{policy_id}", s.handleUpdatePolicy)
	mux.HandleFunc("DELETE /api/v1/tenants/{tenant_id}/policies/{policy_id}", s.handleDeletePolicy)
	mux.HandleFunc("POST /api/v1/tenants/{tenant_id}/policies/{policy_id}/publish", s.handlePublishPolicy)
	mux.HandleFunc("POST /api/v1/tenants/{tenant_id}/simulate", s.handleSimulate)

	// Data Plane Fallback REST endpoints
	mux.HandleFunc("POST /api/v1/decisions", s.handleDecisions)
	mux.HandleFunc("POST /api/v1/decisions/explain", s.handleExplain)

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())

	return mux
}

// StartHTTPServer khởi chạy HTTP server tại cổng chỉ định.
func StartHTTPServer(port int, store *storage.Storage, eng *engine.EngineWithGC, rdb redis.UniversalClient) (*http.Server, error) {
	s := NewHTTPServer(store, eng, rdb)
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

// Handlers implementation

type createPolicyReq struct {
	Effect     string `json:"effect"`
	PolicyText string `json:"policy_text"`
}

func (s *HTTPServer) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		http.Error(w, "Thiếu tenant_id", http.StatusBadRequest)
		return
	}

	var req createPolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON request không hợp lệ", http.StatusBadRequest)
		return
	}

	if req.Effect != "permit" && req.Effect != "forbid" {
		http.Error(w, "Effect không hợp lệ. Chỉ chấp nhận 'permit' hoặc 'forbid'", http.StatusBadRequest)
		return
	}

	id, err := s.storage.CreatePolicy(r.Context(), tenantID, req.Effect, req.PolicyText)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi lưu DB: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"policy_id":"%s","status":"DRAFT"}`, id)))
}

type updatePolicyReq struct {
	PolicyText string `json:"policy_text"`
}

func (s *HTTPServer) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := r.PathValue("policy_id")
	if policyID == "" {
		http.Error(w, "Thiếu policy_id", http.StatusBadRequest)
		return
	}

	var req updatePolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON request không hợp lệ", http.StatusBadRequest)
		return
	}

	err := s.storage.UpdatePolicy(r.Context(), policyID, req.PolicyText)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi DB: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"DRAFT"}`))
}

func (s *HTTPServer) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	policyID := r.PathValue("policy_id")

	err := s.storage.DeletePolicy(r.Context(), policyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi DB: %v", err), http.StatusInternalServerError)
		return
	}

	// Phát đi sự kiện xóa để các PDP node dọn dẹp RAM
	if s.redisClient != nil {
		event := map[string]string{
			"tenant_id": tenantID,
			"policy_id": policyID,
			"action":    "DELETE",
		}
		jsonEvent, _ := json.Marshal(event)
		_ = s.redisClient.Publish(r.Context(), "policy-updates", jsonEvent).Err()
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"Đã xóa chính sách thành công"}`))
}

func (s *HTTPServer) handlePublishPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	policyID := r.PathValue("policy_id")

	// 1. Đọc chính sách DRAFT từ DB
	p, err := s.storage.GetPolicy(r.Context(), policyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Không tìm thấy chính sách: %v", err), http.StatusNotFound)
		return
	}

	// 2. Biên dịch (Compile) và kiểm tra ngữ nghĩa bảo mật của chính sách thô
	lexer := parser.NewLexer(p.PolicyText)
	pr := parser.NewParser(lexer)
	nodes := pr.Parse()
	if len(pr.Errors()) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errJSON, _ := json.Marshal(map[string]interface{}{"errors": pr.Errors()})
		_, _ = w.Write(errJSON)
		return
	}

	compiler := parser.NewCompiler()
	compiled, err := compiler.Compile(nodes[0])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errJSON, _ := json.Marshal(map[string]interface{}{"errors": []string{err.Error()}})
		_, _ = w.Write(errJSON)
		return
	}

	// 3. Serialize AST compiled sang JSON để lưu database
	astJSON, err := json.Marshal(compiled)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi serialize AST: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Cập nhật DB sang ACTIVE
	version, err := s.storage.PublishPolicy(r.Context(), policyID, astJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi DB: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Phát tín hiệu cập nhật qua Redis Pub/Sub đồng bộ cluster PDP nóng
	syncEventSent := false
	if s.redisClient != nil {
		event := map[string]string{
			"tenant_id": tenantID,
			"policy_id": policyID,
			"action":    "UPDATE",
		}
		jsonEvent, _ := json.Marshal(event)
		err = s.redisClient.Publish(r.Context(), "policy-updates", jsonEvent).Err()
		if err == nil {
			syncEventSent = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{
		"policy_id": "%s",
		"status": "ACTIVE",
		"published_version": %d,
		"published_at": "%s",
		"sync_event_sent": %t
	}`, policyID, version, time.Now().Format(time.RFC3339), syncEventSent)))
}

type decisionsReq struct {
	TenantID string            `json:"tenant_id"`
	Subject  string            `json:"subject"`
	Action   string            `json:"action"`
	Resource string            `json:"resource"`
	Context  map[string]string `json:"context"`
}

func (s *HTTPServer) handleDecisions(w http.ResponseWriter, r *http.Request) {
	var req decisionsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON request không hợp lệ", http.StatusBadRequest)
		return
	}

	res := s.engine.CheckPermission(req.TenantID, req.Subject, req.Action, req.Resource, req.Context)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	matchedID := ""
	if len(res.Explanations) > 0 {
		matchedID = res.Explanations[0]
	}
	_, _ = w.Write([]byte(fmt.Sprintf(`{
		"decision": "%s",
		"matched_policy_id": "%s",
		"evaluated_at": "%s"
	}`, res.Decision.String(), matchedID, time.Now().Format(time.RFC3339))))
}

func (s *HTTPServer) handleExplain(w http.ResponseWriter, r *http.Request) {
	var req decisionsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON request không hợp lệ", http.StatusBadRequest)
		return
	}

	res := s.engine.CheckPermission(req.TenantID, req.Subject, req.Action, req.Resource, req.Context)

	// Thu thập giải thích chi tiết
	matchedMetadata := make([]map[string]string, 0)
	trie, exists := s.engine.GetTenantTrie(r.Context(), req.TenantID)
	if exists {
		subjects := trie.RoleDAG.GetInheritedRoles(req.Subject)
		resources := []string{req.Resource}
		actKey := req.Action
		if !strings.HasPrefix(actKey, "action:") {
			actKey = "action:" + actKey
		}

		matchedPolicies := trie.LookupPolicies(subjects, resources, actKey)
		evalCtx := engine.GetEvalContext(req.Subject, req.Action, req.Resource, req.Context, trie.RoleDAG)
		defer evalCtx.Release()

		for _, p := range matchedPolicies {
			val, err := engine.Evaluate(p.Condition, evalCtx)
			satisfied := false
			if err == nil && val.ValType == parser.ValueTypeBool {
				if p.IsUnless {
					satisfied = !val.BoolVal
				} else {
					satisfied = val.BoolVal
				}
			}

			if satisfied {
				effect := "permit"
				if p.Effect == parser.EffectForbid {
					effect = "forbid"
				}
				matchedMetadata = append(matchedMetadata, map[string]string{
					"id":          p.ID,
					"effect":      effect,
					"policy_text": p.PolicyText, // DB text
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	matchedJSON, _ := json.Marshal(matchedMetadata)
	_, _ = w.Write([]byte(fmt.Sprintf(`{
		"decision": "%s",
		"final_reason": "%s",
		"matched_policies": %s
	}`, res.Decision.String(), res.Reason, string(matchedJSON))))
}

}

// simulateReq chứa ngữ cảnh yêu cầu và tập văn bản chính sách cần giả lập.
type simulateReq struct {
	Subject   string            `json:"subject"`
	Action    string            `json:"action"`
	Resource  string            `json:"resource"`
	Context   map[string]string `json:"context"`
	// Danh sách văn bản chính sách DSL cần giả lập (thay vì lấy từ DB)
	Policies []struct {
		ID         string `json:"id"`
		PolicyText string `json:"policy_text"`
	} `json:"policies"`
	// Tùy chọn: Giả lập thêm cả chính sách ACTIVE trên DB
	IncludeActive bool `json:"include_active"`
}

// handleSimulate thực hiện giả lập kiểm tra quyền trên một tập chính sách tạm thời
// mà không ảnh hưởng đến bộ nhớ RAM Engine đang phục vụ.
func (s *HTTPServer) handleSimulate(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		http.Error(w, "Thiếu tenant_id", http.StatusBadRequest)
		return
	}

	var req simulateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON request không hợp lệ", http.StatusBadRequest)
		return
	}

	// 1. Tạo Trie tạm thời hoàn toàn độc lập với Engine chính
	tempTrie := engine.NewTrieRoot(tenantID)
	compiler := parser.NewCompiler()
	compileErrors := make([]string, 0)

	// 2. Biên dịch các chính sách DSL được gửi lên
	for _, p := range req.Policies {
		lexer := parser.NewLexer(p.PolicyText)
		pr := parser.NewParser(lexer)
		nodes := pr.Parse()

		if len(pr.Errors()) > 0 {
			compileErrors = append(compileErrors, fmt.Sprintf("[%s]: %v", p.ID, pr.Errors()))
			continue
		}

		nodes[0].ID = p.ID
		compiled, err := compiler.Compile(nodes[0])
		if err != nil {
			compileErrors = append(compileErrors, fmt.Sprintf("[%s]: %v", p.ID, err))
			continue
		}
		tempTrie.AddPolicy(compiled)
	}

	// 3. Tùy chọn: Nạp thêm chính sách ACTIVE trên DB vào Trie tạm thời
	if req.IncludeActive {
		dbPolicies, err := s.storage.GetActivePolicies(r.Context(), tenantID)
		if err == nil {
			for _, dbP := range dbPolicies {
				lexer := parser.NewLexer(dbP.PolicyText)
				pr := parser.NewParser(lexer)
				nodes := pr.Parse()
				if len(pr.Errors()) > 0 {
					continue
				}
				nodes[0].ID = dbP.ID
				compiled, err := compiler.Compile(nodes[0])
				if err != nil {
					continue
				}
				tempTrie.AddPolicy(compiled)
			}
		}
	}

	// 4. Chạy đánh giá quyết định trên Trie tạm thời (hoàn toàn không ảnh hưởng Engine chính)
	result := engine.CheckPermission(tempTrie, req.Subject, req.Action, req.Resource, req.Context)

	// 5. Trả về kết quả giả lập
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	errJSON, _ := json.Marshal(compileErrors)
	exJSON, _ := json.Marshal(result.Explanations)
	_, _ = w.Write([]byte(fmt.Sprintf(`{
		"simulated_decision": "%s",
		"reason": "%s",
		"matched_policies": %s,
		"compile_errors": %s,
		"tenant_id": "%s"
	}`, result.Decision.String(), result.Reason, string(exJSON), string(errJSON), tenantID)))
}

import "strings"

