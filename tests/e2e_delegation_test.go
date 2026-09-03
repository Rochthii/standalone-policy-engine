package tests

import (
	"context"
	"os"
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

// setupE2ETestEngine khởi tạo máy chủ PDP hoàn chỉnh với 6 luật P2P hạt giống từ configs/policies.cedar
func setupE2ETestEngine(t *testing.T) (*server.GRPCServer, *engine.EngineWithGC) {
	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	compiler := parser.NewCompiler()

	cedarContent, err := os.ReadFile("../configs/policies.cedar")
	if err != nil {
		t.Fatalf("Không thể đọc file configs/policies.cedar: %v", err)
	}

	lexer := parser.NewLexer(string(cedarContent))
	p := parser.NewParser(lexer)
	rawPolicies := p.Parse()
	if len(rawPolicies) == 0 {
		t.Fatalf("Parse policies.cedar thất bại: không tìm thấy policy nào")
	}

	var compiledPolicies []*parser.PolicyNode
	for idx, raw := range rawPolicies {
		raw.ID = "POL-P2P-" + strconv.Itoa(idx+1)
		compiled, err := compiler.Compile(raw)
		if err != nil {
			t.Fatalf("Compile policy %s thất bại: %v", raw.ID, err)
		}
		compiledPolicies = append(compiledPolicies, compiled)
	}

	// Đăng ký quan hệ thừa kế vai trò: Manager kế thừa Staff
	inheritances := [][2]string{
		{"role:department_manager", "role:staff"},
		{"user:manager_bob", "role:department_manager"},
		{"user:staff_alice", "role:staff"},
	}

	err = eng.UpdateTenantPoliciesWithRevision("tenant-odoo", compiledPolicies, inheritances, 1)
	if err != nil {
		t.Fatalf("Update tenant policies thất bại: %v", err)
	}

	srv := server.NewGRPCServer(eng, nil)
	return srv, eng
}

// TestE2E_P2P_Delegation_7Vectors kiểm thử toàn diện 7 test vectors theo tài liệu BENCHMARK_REPRODUCIBILITY.md
func TestE2E_P2P_Delegation_7Vectors(t *testing.T) {
	srv, _ := setupE2ETestEngine(t)
	delegationMgr := security.NewDelegationManager()
	ctx := context.Background()

	validUntil := strconv.FormatInt(time.Now().Add(2*time.Hour).Unix(), 10)
	pastUntil := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)

	// ========================================================================
	// TC-01: Manager tạo PO và tự duyệt (SoD Collision) -> DENY
	// ========================================================================
	t.Run("TC-01: Manager self-approves own PO -> DENY (SoD)", func(t *testing.T) {
		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  "user:manager_bob",
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-001",
			Context: map[string]string{
				"amount":               "4500",
				"delegation_chain":     "user:manager_bob",
				"resource.creator_id":  "user:manager_bob",
				"principal.department": "Procurement",
				"resource.department":  "Procurement",
			},
		}
		res, err := srv.CheckAccess(ctx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Decision != policyv1.CheckAccessResponse_DENY {
			t.Fatalf("TC-01 FAILED: Expected DENY due to SoD, got: %v", res.Decision)
		}
	})

	// ========================================================================
	// TC-02: AI Agent duyệt hộ PO do Manager tạo (Confused Deputy SoD) -> DENY
	// ========================================================================
	t.Run("TC-02: AI Agent approves PO created by Delegator -> DENY (SoD Chain)", func(t *testing.T) {
		grantID := "grant-tc02"
		delegator := "user:manager_bob"
		agent := "agent:procurement_copilot"
		amount := "1800"
		proof := delegationMgr.GenerateProof(grantID, delegator, agent, amount, validUntil)

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  agent,
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-002",
			Context: map[string]string{
				"amount":                 amount,
				"delegation_grant_id":    grantID,
				"delegated_by":           delegator,
				"delegation_valid_until": validUntil,
				"delegation_proof":       proof,
				"delegation_chain":       "user:manager_bob,agent:procurement_copilot",
				"resource.creator_id":    "user:manager_bob", // Trùng với Delegator trong chuỗi
				"tool_context":           "tool:auto_confirm_po",
				"execution_mode":         "autonomous_run",
			},
		}
		res, err := srv.CheckAccess(ctx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Decision != policyv1.CheckAccessResponse_DENY {
			t.Fatalf("TC-02 FAILED: Expected DENY due to delegation chain SoD violation, got: %v", res.Decision)
		}
	})

	// ========================================================================
	// TC-03: AI Agent tự động duyệt PO hợp lệ (<= 2,000 USD) -> ALLOW
	// ========================================================================
	t.Run("TC-03: AI Agent autonomous PO approval <= $2,000 -> ALLOW", func(t *testing.T) {
		grantID := "grant-tc03"
		delegator := "user:manager_bob"
		agent := "agent:procurement_copilot"
		amount := "1500"
		proof := delegationMgr.GenerateProof(grantID, delegator, agent, amount, validUntil)

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  agent,
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-003",
			Context: map[string]string{
				"amount":                 amount,
				"delegation_grant_id":    grantID,
				"delegated_by":           delegator,
				"delegation_valid_until": validUntil,
				"delegation_proof":       proof,
				"delegation_chain":       "user:manager_bob,agent:procurement_copilot",
				"resource.creator_id":    "user:staff_alice", // Alice tạo -> Hợp lệ SoD
				"tool_context":           "tool:auto_confirm_po",
				"execution_mode":         "autonomous_run",
			},
		}
		res, err := srv.CheckAccess(ctx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Decision != policyv1.CheckAccessResponse_ALLOW {
			t.Fatalf("TC-03 FAILED: Expected ALLOW, got: %v", res.Decision)
		}
	})

	// ========================================================================
	// TC-04: AI Agent duyệt PO vượt trần tự hành (> 2,000 USD) -> DENY (Forbid Override)
	// ========================================================================
	t.Run("TC-04: AI Agent approves PO > $2,000 -> DENY (Guardrail Limit)", func(t *testing.T) {
		grantID := "grant-tc04"
		delegator := "user:manager_bob"
		agent := "agent:procurement_copilot"
		amount := "3500"
		proof := delegationMgr.GenerateProof(grantID, delegator, agent, amount, validUntil)

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  agent,
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-004",
			Context: map[string]string{
				"amount":                 amount,
				"delegation_grant_id":    grantID,
				"delegated_by":           delegator,
				"delegation_valid_until": validUntil,
				"delegation_proof":       proof,
				"delegation_chain":       "user:manager_bob,agent:procurement_copilot",
				"resource.creator_id":    "user:staff_alice",
				"tool_context":           "tool:auto_confirm_po",
				"execution_mode":         "autonomous_run",
			},
		}
		res, err := srv.CheckAccess(ctx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Decision != policyv1.CheckAccessResponse_DENY {
			t.Fatalf("TC-04 FAILED: Expected DENY for amount > 2000, got: %v", res.Decision)
		}
	})

	// ========================================================================
	// TC-05: AI Agent duyệt PO với HMAC bị sửa đổi -> 403 PermissionDenied
	// ========================================================================
	t.Run("TC-05: Tampered Amount in Proof -> 403 PermissionDenied", func(t *testing.T) {
		grantID := "grant-tc05"
		delegator := "user:manager_bob"
		agent := "agent:procurement_copilot"
		// Proof được ký cho $1,500
		proof := delegationMgr.GenerateProof(grantID, delegator, agent, "1500", validUntil)

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  agent,
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-005",
			Context: map[string]string{
				"amount":                 "50000", // Kẻ tấn công sửa context.amount thành 50,000
				"delegation_grant_id":    grantID,
				"delegated_by":           delegator,
				"delegation_valid_until": validUntil,
				"delegation_proof":       proof,
				"delegation_chain":       "user:manager_bob,agent:procurement_copilot",
				"resource.creator_id":    "user:staff_alice",
				"tool_context":           "tool:auto_confirm_po",
				"execution_mode":         "autonomous_run",
			},
		}
		_, err := srv.CheckAccess(ctx, req)
		if err == nil {
			t.Fatalf("TC-05 FAILED: Expected error for tampered HMAC proof, but got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.PermissionDenied {
			t.Fatalf("TC-05 FAILED: Expected code PermissionDenied, got: %v", st.Code())
		}
	})

	// ========================================================================
	// TC-06: AI Agent duyệt PO khi Grant đã bị Revoke trên RAM -> DENY
	// ========================================================================
	t.Run("TC-06: Revoked Grant on RAM -> DENY (TOCTOU Defense)", func(t *testing.T) {
		grantID := "grant-revoked-ram"
		delegator := "user:manager_bob"
		agent := "agent:procurement_copilot"
		amount := "1200"
		proof := delegationMgr.GenerateProof(grantID, delegator, agent, amount, validUntil)

		// Manager bấm thu hồi qua gRPC RPC
		revokeReq := &policyv1.RevokeRequest{
			TenantId:  "tenant-odoo",
			GrantId:   grantID,
			RevokedBy: "user:manager_bob",
			Reason:    "Kích hoạt quy trình khẩn cấp thu hồi ủy quyền",
		}
		_, err := srv.RevokeDelegation(ctx, revokeReq)
		if err != nil {
			t.Fatalf("RevokeDelegation failed: %v", err)
		}

		// Agent tiếp tục gửi yêu cầu
		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  agent,
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-006",
			Context: map[string]string{
				"amount":                 amount,
				"delegation_grant_id":    grantID,
				"delegated_by":           delegator,
				"delegation_valid_until": validUntil,
				"delegation_proof":       proof,
				"delegation_chain":       "user:manager_bob,agent:procurement_copilot",
				"resource.creator_id":    "user:staff_alice",
				"tool_context":           "tool:auto_confirm_po",
				"execution_mode":         "autonomous_run",
			},
		}
		res, err := srv.CheckAccess(ctx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Decision != policyv1.CheckAccessResponse_DENY {
			t.Fatalf("TC-06 FAILED: Expected DENY after Revoke, got: %v", res.Decision)
		}
		if res.MatchedPolicyId != "POL-REVOCATION-BLACK-LIST" {
			t.Fatalf("TC-06 FAILED: Expected MatchedPolicyId POL-REVOCATION-BLACK-LIST, got: %v", res.MatchedPolicyId)
		}
	})

	// ========================================================================
	// TC-07: AI Agent duyệt PO với Token hết hạn TTL -> 403 PermissionDenied
	// ========================================================================
	t.Run("TC-07: Expired TTL Proof -> 403 PermissionDenied", func(t *testing.T) {
		grantID := "grant-expired-ttl"
		delegator := "user:manager_bob"
		agent := "agent:procurement_copilot"
		amount := "1200"
		proof := delegationMgr.GenerateProof(grantID, delegator, agent, amount, pastUntil)

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-odoo",
			Subject:  agent,
			Action:   "action:APPROVE_PURCHASE_ORDER",
			Resource: "purchase_order:PO-2026-007",
			Context: map[string]string{
				"amount":                 amount,
				"delegation_grant_id":    grantID,
				"delegated_by":           delegator,
				"delegation_valid_until": pastUntil,
				"delegation_proof":       proof,
				"delegation_chain":       "user:manager_bob,agent:procurement_copilot",
				"resource.creator_id":    "user:staff_alice",
				"tool_context":           "tool:auto_confirm_po",
				"execution_mode":         "autonomous_run",
			},
		}
		_, err := srv.CheckAccess(ctx, req)
		if err == nil {
			t.Fatalf("TC-07 FAILED: Expected error for expired TTL proof, but got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.PermissionDenied {
			t.Fatalf("TC-07 FAILED: Expected PermissionDenied (403), got: %v", st.Code())
		}
	})
}
