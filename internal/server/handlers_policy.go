package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"standalone-policy-engine/internal/parser"
)

type createPolicyRequest struct {
	Effect     string `json:"effect"`
	PolicyText string `json:"policy_text"`
}

type updatePolicyRequest struct {
	PolicyText string `json:"policy_text"`
}

func (s *HTTPServer) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		http.Error(w, "Thiếu tenant_id trong path", http.StatusBadRequest)
		return
	}

	var req createPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dữ liệu JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	effect := strings.ToUpper(strings.TrimSpace(req.Effect))
	if effect != "PERMIT" && effect != "FORBID" {
		http.Error(w, "Effect chỉ được là PERMIT hoặc FORBID", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.PolicyText) == "" {
		http.Error(w, "PolicyText không được để trống", http.StatusBadRequest)
		return
	}

	id, err := s.storage.CreatePolicy(r.Context(), tenantID, effect, req.PolicyText)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi tạo policy: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"%s","policy_id":"%s","status":"DRAFT","message":"Tạo chính sách thành công"}`, id, id)))
}

func (s *HTTPServer) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := r.PathValue("policy_id")
	if policyID == "" {
		http.Error(w, "Thiếu policy_id trong path", http.StatusBadRequest)
		return
	}

	var req updatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dữ liệu JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.PolicyText) == "" {
		http.Error(w, "PolicyText không được để trống", http.StatusBadRequest)
		return
	}

	err := s.storage.UpdatePolicy(r.Context(), policyID, req.PolicyText)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi cập nhật policy: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"DRAFT","message":"Cập nhật chính sách thành công"}`))
}

func (s *HTTPServer) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := r.PathValue("policy_id")
	if policyID == "" {
		http.Error(w, "Thiếu policy_id trong path", http.StatusBadRequest)
		return
	}

	err := s.storage.DeletePolicy(r.Context(), policyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi xóa policy: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"Xóa chính sách thành công"}`))
}

func (s *HTTPServer) handlePublishPolicy(w http.ResponseWriter, r *http.Request) {
	policyID := r.PathValue("policy_id")
	if policyID == "" {
		http.Error(w, "Thiếu policy_id trong path", http.StatusBadRequest)
		return
	}

	p, err := s.storage.GetPolicy(r.Context(), policyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Không tìm thấy policy: %v", err), http.StatusNotFound)
		return
	}

	lexer := parser.NewLexer(p.PolicyText)
	pParser := parser.NewParser(lexer)
	nodes := pParser.Parse()

	if len(pParser.Errors()) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errsJSON, _ := json.Marshal(pParser.Errors())
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"COMPILE_ERROR","errors":%s}`, string(errsJSON))))
		return
	}

	compiler := parser.NewCompiler()
	nodes[0].ID = policyID
	compiledNode, err := compiler.Compile(nodes[0])
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi compiler: %v", err), http.StatusBadRequest)
		return
	}

	astJSON, err := json.Marshal(compiledNode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi serialize AST: %v", err), http.StatusInternalServerError)
		return
	}

	newVersion, err := s.storage.PublishPolicy(r.Context(), policyID, astJSON)
	if err != nil {
		http.Error(w, fmt.Sprintf("Lỗi lưu DB: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"ACTIVE","version":%d,"message":"Xuất bản chính sách thành công"}`, newVersion)))
}
