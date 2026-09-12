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

	bundle, err := s.storage.GetTenantPolicyBundle(r.Context(), tenantID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi truy vấn DB: %v", err), http.StatusInternalServerError)
		return
	}

	sources := make([]parser.PolicySource, 0, len(bundle.Policies))
	for _, dbP := range bundle.Policies {
		sources = append(sources, parser.PolicySource{ID: dbP.ID, Text: dbP.PolicyText})
	}
	compiledPolicies, err := parser.CompilePolicySet(sources)
	if err != nil {
		http.Error(w, fmt.Sprintf("Không thể prewarm ruleset không hợp lệ: %v", err), http.StatusUnprocessableEntity)
		return
	}

	err = s.engine.UpdateTenantPoliciesWithRevision(tenantID, compiledPolicies, bundle.Inheritances, bundle.Revision)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi nạp RAM: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"PREWARMED","tenant_id":"%s","revision":%d,"active_policies":%d}`, tenantID, bundle.Revision, len(compiledPolicies))))
}
