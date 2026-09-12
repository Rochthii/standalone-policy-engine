package tests

import (
	"strconv"
	"testing"
	"time"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/server"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupTestServerWithEngine(t *testing.T) (*server.GRPCServer, *engine.EngineWithGC) {
	configureIntegrationTestJWT(t)
	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	compiler := parser.NewCompiler()

	// Seed 1 policy đơn giản: Cho phép agent tự hành nếu amount <= 2000
	policyDSL := `
		permit(
			principal == agent:procurement_copilot,
			action == action:APPROVE_PURCHASE_ORDER,
			resource == any
		)
		when {
			context.amount <= 2000
		};
	`
	p := compileHelper(t, compiler, "POL-AGENT-AUTONOMOUS-ALLOW", policyDSL)
	err := eng.UpdateTenantPoliciesWithRevision("tenant-corp", []*parser.PolicyNode{p}, nil, 1)
	if err != nil {
		t.Fatalf("failed to update test policies: %v", err)
	}

	srv := server.NewGRPCServer(eng, nil)
	return srv, eng
}

func TestDelegationInterceptor_ValidProof_Allowed(t *testing.T) {
	srv, _ := setupTestServerWithEngine(t)
	delegationMgr := security.NewDelegationManager()

	grantID := "grant-101"
	delegator := "user:manager_bob"
	agent := "agent:procurement_copilot"
	amount := "1500"
	validUntil := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)

	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-corp",
		Subject:  agent,
		Action:   "action:APPROVE_PURCHASE_ORDER",
		Resource: "purchase_order:PO-001",
		Context: signedDelegationContext(t, delegationMgr, "tenant-corp", grantID, delegator, agent,
			"action:APPROVE_PURCHASE_ORDER", "purchase_order:PO-001", amount, validUntil, "user:alice"),
	}

	res, err := srv.CheckAccess(authenticatedIncomingContext(t, "tenant-corp", agent), req)
	if err != nil {
		t.Fatalf("CheckAccess returned unexpected error: %v", err)
	}

	if res.Decision != policyv1.CheckAccessResponse_ALLOW {
		t.Fatalf("expected decision ALLOW, got: %v", res.Decision)
	}
}

func TestDelegationInterceptor_TamperedProof_Denied(t *testing.T) {
	srv, _ := setupTestServerWithEngine(t)

	grantID := "grant-101"
	delegator := "user:manager_bob"
	agent := "agent:procurement_copilot"
	amount := "1500"
	validUntil := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)
	tamperedProof := "invalid_hmac_hex_string_32_bytes_long_12345678"
	delegationMgr := security.NewDelegationManager()
	contextValues := signedDelegationContext(t, delegationMgr, "tenant-corp", grantID, delegator, agent,
		"action:APPROVE_PURCHASE_ORDER", "purchase_order:PO-001", amount, validUntil, "user:alice")
	contextValues["delegation_proof"] = tamperedProof

	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-corp",
		Subject:  agent,
		Action:   "action:APPROVE_PURCHASE_ORDER",
		Resource: "purchase_order:PO-001",
		Context:  contextValues,
	}

	_, err := srv.CheckAccess(authenticatedIncomingContext(t, "tenant-corp", agent), req)
	if err == nil {
		t.Fatalf("expected error for tampered proof, but got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected status code PermissionDenied (403), got: %v", st.Code())
	}
}

func TestDelegationInterceptor_CompleteTupleIsRequired(t *testing.T) {
	srv, _ := setupTestServerWithEngine(t)
	delegationMgr := security.NewDelegationManager()
	grantID := "grant-complete-tuple"
	delegator := "user:manager_bob"
	agent := "agent:procurement_copilot"
	action := "action:APPROVE_PURCHASE_ORDER"
	resource := "purchase_order:PO-001"
	validUntil := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)

	tests := []struct {
		name   string
		mutate func(*policyv1.CheckAccessRequest)
	}{
		{name: "missing proof", mutate: func(req *policyv1.CheckAccessRequest) { delete(req.Context, "delegation_proof") }},
		{name: "altered action", mutate: func(req *policyv1.CheckAccessRequest) { req.Action = "action:DELETE_PURCHASE_ORDER" }},
		{name: "altered resource", mutate: func(req *policyv1.CheckAccessRequest) { req.Resource = "purchase_order:PO-999" }},
		{name: "altered creator", mutate: func(req *policyv1.CheckAccessRequest) { req.Context["resource.creator_id"] = delegator }},
		{name: "altered chain", mutate: func(req *policyv1.CheckAccessRequest) { req.Context["delegation_chain"] = "user:eve," + agent }},
		{name: "missing nonce", mutate: func(req *policyv1.CheckAccessRequest) { delete(req.Context, "delegation_nonce") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &policyv1.CheckAccessRequest{
				TenantId: "tenant-corp",
				Subject:  agent,
				Action:   action,
				Resource: resource,
				Context: signedDelegationContext(t, delegationMgr, "tenant-corp", grantID, delegator, agent,
					action, resource, "1500", validUntil, "user:alice"),
			}
			test.mutate(req)
			_, err := srv.CheckAccess(authenticatedIncomingContext(t, "tenant-corp", agent), req)
			if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("expected PermissionDenied, got %v", err)
			}
		})
	}
}

func TestDelegationInterceptor_ExpiredProof_Denied(t *testing.T) {
	srv, _ := setupTestServerWithEngine(t)
	delegationMgr := security.NewDelegationManager()

	grantID := "grant-101"
	delegator := "user:manager_bob"
	agent := "agent:procurement_copilot"
	amount := "1500"
	pastTime := strconv.FormatInt(time.Now().Add(-5*time.Minute).Unix(), 10)

	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-corp",
		Subject:  agent,
		Action:   "action:APPROVE_PURCHASE_ORDER",
		Resource: "purchase_order:PO-001",
		Context: signedDelegationContext(t, delegationMgr, "tenant-corp", grantID, delegator, agent,
			"action:APPROVE_PURCHASE_ORDER", "purchase_order:PO-001", amount, pastTime, "user:alice"),
	}

	_, err := srv.CheckAccess(authenticatedIncomingContext(t, "tenant-corp", agent), req)
	if err == nil {
		t.Fatalf("expected error for expired proof, but got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected status code PermissionDenied (403), got: %v", st.Code())
	}
}

func TestDelegationInterceptor_Revocation_TOCTOU_Defense(t *testing.T) {
	srv, _ := setupTestServerWithEngine(t)
	delegationMgr := security.NewDelegationManager()

	grantID := "grant-to-be-revoked"
	delegator := "user:manager_bob"
	agent := "agent:procurement_copilot"
	amount := "1500"
	validUntil := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)

	// 1. Kiểm tra khi chưa Revoke -> ALLOW
	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-corp",
		Subject:  agent,
		Action:   "action:APPROVE_PURCHASE_ORDER",
		Resource: "purchase_order:PO-001",
		Context: signedDelegationContext(t, delegationMgr, "tenant-corp", grantID, delegator, agent,
			"action:APPROVE_PURCHASE_ORDER", "purchase_order:PO-001", amount, validUntil, "user:alice"),
	}

	res1, err := srv.CheckAccess(authenticatedIncomingContext(t, "tenant-corp", agent), req)
	if err != nil || res1.Decision != policyv1.CheckAccessResponse_ALLOW {
		t.Fatalf("initial request should be ALLOW, got: %v, err: %v", res1.Decision, err)
	}

	// 2. Manager bấm nút Revoke từ Odoo UI (gọi RevokeDelegation RPC)
	revokeReq := &policyv1.RevokeRequest{
		TenantId:  "tenant-corp",
		GrantId:   grantID,
		RevokedBy: "user:manager_bob",
		Reason:    "Suspected prompt injection incident",
	}
	revokeRes, err := srv.RevokeDelegation(authenticatedIncomingContextWithPermissions(t, "tenant-corp", delegator, "delegation:revoke"), revokeReq)
	if err != nil || !revokeRes.Success {
		t.Fatalf("RevokeDelegation failed: %v", err)
	}

	// 3. Agent gửi request ngay lập tức -> Phải bị từ chối DENY với mã POL-REVOCATION-BLACK-LIST
	res2, err := srv.CheckAccess(authenticatedIncomingContext(t, "tenant-corp", agent), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Decision != policyv1.CheckAccessResponse_DENY {
		t.Fatalf("expected decision DENY after revocation, got: %v", res2.Decision)
	}
	if res2.MatchedPolicyId != "POL-REVOCATION-BLACK-LIST" {
		t.Fatalf("expected MatchedPolicyId POL-REVOCATION-BLACK-LIST, got: %v", res2.MatchedPolicyId)
	}
}
