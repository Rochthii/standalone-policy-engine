package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
)

type DecisionRequest struct {
	TenantID string            `json:"tenant_id"`
	Subject  string            `json:"subject"`
	Action   string            `json:"action"`
	Resource string            `json:"resource"`
	Context  map[string]string `json:"context"`
}

type SimulateRequest struct {
	PolicyText string            `json:"policy_text"`
	Subject    string            `json:"subject"`
	Action     string            `json:"action"`
	Resource   string            `json:"resource"`
	Context    map[string]string `json:"context"`
}

func (s *HTTPServer) handleDecisions(w http.ResponseWriter, r *http.Request) {
	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	identity, ok := validateDecisionIdentity(w, r, req)
	if !ok {
		return
	}
	trustedContext := mergeTrustedPrincipalContext(req.Context, identity.PrincipalAttributes)
	result := s.engine.CheckPermission(r.Context(), identity.TenantID, identity.Subject, req.Action, req.Resource, trustedContext)

	matchedID := ""
	if len(result.Explanations) > 0 {
		matchedID = result.Explanations[0]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Decision        string              `json:"decision"`
		MatchedPolicyID string              `json:"matched_policy_id,omitempty"`
		Obligations     []engine.Obligation `json:"obligations,omitempty"`
	}{
		Decision:        result.Decision.String(),
		MatchedPolicyID: matchedID,
		Obligations:     result.Obligations,
	})
}

func (s *HTTPServer) handleExplain(w http.ResponseWriter, r *http.Request) {
	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	identity, ok := validateDecisionIdentity(w, r, req)
	if !ok {
		return
	}
	trustedContext := mergeTrustedPrincipalContext(req.Context, identity.PrincipalAttributes)
	result := s.engine.CheckPermission(r.Context(), identity.TenantID, identity.Subject, req.Action, req.Resource, trustedContext)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Decision        string              `json:"decision"`
		FinalReason     string              `json:"final_reason"`
		MatchedPolicies []string            `json:"matched_policies"`
		Obligations     []engine.Obligation `json:"obligations,omitempty"`
	}{
		Decision:        result.Decision.String(),
		FinalReason:     result.Reason,
		MatchedPolicies: result.Explanations,
		Obligations:     result.Obligations,
	})
}

func validateDecisionIdentity(w http.ResponseWriter, r *http.Request, req DecisionRequest) (authenticatedIdentity, bool) {
	identity, ok := identityFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"authenticated identity is missing"}`, http.StatusUnauthorized)
		return authenticatedIdentity{}, false
	}
	if req.TenantID != "" && req.TenantID != identity.TenantID {
		http.Error(w, `{"error":"tenant_id in body does not match token"}`, http.StatusForbidden)
		return authenticatedIdentity{}, false
	}
	if req.Subject != "" && req.Subject != identity.Subject {
		http.Error(w, `{"error":"subject in body does not match token"}`, http.StatusForbidden)
		return authenticatedIdentity{}, false
	}
	return identity, true
}

func (s *HTTPServer) handleSimulate(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenant_id")
	if tenantID == "" {
		http.Error(w, "Thiếu tenant_id trong path", http.StatusBadRequest)
		return
	}

	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	var simPolicy *parser.PolicyNode
	var compileErrors []string

	if req.PolicyText != "" {
		lexer := parser.NewLexer(req.PolicyText)
		pParser := parser.NewParser(lexer)
		nodes := pParser.Parse()

		if len(pParser.Errors()) > 0 {
			compileErrors = pParser.Errors()
		} else if len(nodes) > 0 {
			compiler := parser.NewCompiler()
			nodes[0].ID = "SIMULATED_CANDIDATE"
			compiled, err := compiler.Compile(nodes[0])
			if err != nil {
				compileErrors = []string{err.Error()}
			} else {
				simPolicy = compiled
			}
		}
	}

	result := s.engine.SimulateDecision(r.Context(), tenantID, simPolicy, req.Subject, req.Action, req.Resource, req.Context)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	exJSON, _ := json.Marshal(result.Explanations)
	errJSON, _ := json.Marshal(compileErrors)

	_, _ = w.Write([]byte(fmt.Sprintf(`{
		"decision": "%s",
		"reason": "%s",
		"matched_policies": %s,
		"compile_errors": %s,
		"tenant_id": "%s"
	}`, result.Decision.String(), result.Reason, string(exJSON), string(errJSON), tenantID)))
}
