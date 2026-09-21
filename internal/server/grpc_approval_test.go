package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func approvalTestServer(t *testing.T) *GRPCServer {
	t.Helper()
	configureGRPCTestJWT(t)
	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	lexer := parser.NewLexer(`permit(principal == user:approver, action == action:APPROVE_PURCHASE_ORDER, resource == any) when { context.approval_state == "pending" && principal.department == resource.department };`)
	policyParser := parser.NewParser(lexer)
	nodes := policyParser.Parse()
	if len(nodes) != 1 || len(policyParser.Errors()) != 0 {
		t.Fatalf("parse approval policy: %v", policyParser.Errors())
	}
	nodes[0].ID = "approval-policy"
	compiled, err := parser.NewCompiler().Compile(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.UpdateTenantPoliciesWithRevision("tenant-a", []*parser.PolicyNode{compiled}, nil, 7); err != nil {
		t.Fatal(err)
	}
	manager, err := security.NewApprovalCapabilityManagerWithKeyring(
		"approval-2026",
		map[string]string{"approval-2026": "approval-server-test-secret-at-least-32-characters"},
		15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	server := NewGRPCServer(eng, nil)
	server.approvalMgr = manager
	server.delegationMgr.SetRevocationReady(true)
	return server
}

func approvalIssueRequest() *policyv1.IssueApprovalCapabilityRequest {
	return &policyv1.IssueApprovalCapabilityRequest{
		TenantId:             "tenant-a",
		CompanyId:            17,
		ApprovalId:           base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16)),
		IntentHash:           strings.Repeat("a", 64),
		StateWitness:         strings.Repeat("b", 64),
		CommandId:            base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)),
		DelegationGrantId:    42,
		DelegatorSubject:     "user:delegator",
		AgentSubject:         "agent:procurement",
		CreatorSubject:       "user:creator",
		ApproverUserId:       77,
		Resource:             "purchase_order:99",
		DelegationValidUntil: time.Now().Add(time.Hour).Unix(),
		Context:              map[string]string{"resource.department": "Procurement"},
	}
}

func approvalHumanContext(t *testing.T, subject string) context.Context {
	t.Helper()
	return incomingJWTContextWithClaims(t, jwt.MapClaims{
		"sub":        subject,
		"tenant_id":  "tenant-a",
		"department": "Procurement",
		"company_id": "17",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
}

func TestApprovalCapabilityRPCIssueAndVerify(t *testing.T) {
	server := approvalTestServer(t)
	ctx := approvalHumanContext(t, "user:approver")
	response, err := server.IssueApprovalCapability(ctx, approvalIssueRequest())
	if err != nil {
		t.Fatal(err)
	}
	capability := response.Capability
	if capability == nil || capability.IssuancePolicyRevision != 7 || capability.ApproverSubject != "user:approver" || capability.Algorithm != security.ApprovalCapabilityAlgorithm {
		t.Fatalf("unexpected approval capability: %+v", capability)
	}
	verified, err := server.VerifyApprovalCapability(ctx, &policyv1.VerifyApprovalCapabilityRequest{TenantId: "tenant-a", Capability: capability})
	if err != nil || !verified.Valid {
		t.Fatalf("verify issued capability: response=%+v err=%v", verified, err)
	}
}

func TestApprovalCapabilityRPCFailsClosed(t *testing.T) {
	server := approvalTestServer(t)
	humanContext := approvalHumanContext(t, "user:approver")
	response, err := server.IssueApprovalCapability(humanContext, approvalIssueRequest())
	if err != nil {
		t.Fatal(err)
	}
	tampered := proto.Clone(response.Capability).(*policyv1.ApprovalCapability)
	tampered.IntentHash = strings.Repeat("c", 64)
	_, err = server.VerifyApprovalCapability(humanContext, &policyv1.VerifyApprovalCapabilityRequest{TenantId: "tenant-a", Capability: tampered})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("tampered capability status=%v err=%v", status.Code(err), err)
	}
	wrongKey := proto.Clone(response.Capability).(*policyv1.ApprovalCapability)
	wrongKey.KeyId = "delegation-2026"
	_, err = server.VerifyApprovalCapability(humanContext, &policyv1.VerifyApprovalCapabilityRequest{TenantId: "tenant-a", Capability: wrongKey})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("key-confused capability status=%v err=%v", status.Code(err), err)
	}
	_, err = server.IssueApprovalCapability(approvalHumanContext(t, "agent:procurement"), approvalIssueRequest())
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("agent issuance status=%v err=%v", status.Code(err), err)
	}
}
