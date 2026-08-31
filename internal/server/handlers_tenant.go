package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"standalone-policy-engine/internal/parser"
)

// handleGetTenantSchema trả về danh sách các trường context mà Tenant thực sự yêu cầu (Attribute Schema).
func (s *HTTPServer) handleGetTenantSchema(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		http.Error(w, "Thiếu tenant_id trong path", http.StatusBadRequest)
		return
	}

	attrs := s.engine.GetTenantSchema(tenantID)
	revision := s.engine.GetTenantRevision(tenantID)
	if attrs == nil {
		attrs = []string{}
	}

	resp := map[string]interface{}{
		"tenant_id":           tenantID,
		"revision":            revision,
		"required_attributes": attrs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handlePrewarm nạp trước chính sách của Tenant vào RAM (hóa giải hiện tượng Cold Start cho VIP Tenants).
func (s *HTTPServer) handlePrewarm(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		http.Error(w, "Thiếu tenant_id trong path", http.StatusBadRequest)
		return
	}

	dbPolicies, err := s.storage.GetActivePolicies(r.Context(), tenantID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi truy vấn DB: %v", err), http.StatusInternalServerError)
		return
	}

	compiler := parser.NewCompiler()
	compiledPolicies := make([]*parser.PolicyNode, 0, len(dbPolicies))
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
		compiledPolicies = append(compiledPolicies, compiled)
	}

	rev, _ := s.storage.GetTenantRevision(r.Context(), tenantID)
	if rev == 0 {
		rev = 1
	}

	err = s.engine.UpdateTenantPoliciesWithRevision(tenantID, compiledPolicies, nil, rev)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi nạp RAM: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"PREWARMED","tenant_id":"%s","revision":%d,"active_policies":%d}`, tenantID, rev, len(compiledPolicies))))
}
