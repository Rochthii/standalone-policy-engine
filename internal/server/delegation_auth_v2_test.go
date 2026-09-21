package server

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"
)

func TestValidateDelegationV2BindsHighImpactRequest(t *testing.T) {
	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	req := signedV2PurchaseOrderRequest(t, srv)
	req.Context["amount"] = "1"
	req.Context["tool_context"] = "tool:forged"
	req.Context["execution_mode"] = "forged"

	if err := srv.validateDelegation(req); err != nil {
		t.Fatalf("valid V2 request was rejected: %v", err)
	}
	if req.Context["amount"] != "1234.56" || req.Context["amount_minor"] != "123456" {
		t.Fatalf("policy amount was not derived from CBI: %#v", req.Context)
	}
	if req.Context["tool_context"] != "tool:auto_confirm_po" || req.Context["execution_mode"] != "autonomous_run" {
		t.Fatalf("route-owned policy context was not restored: %#v", req.Context)
	}
}

func TestValidateDelegationV2RejectsDowngradeAndMaterialTampering(t *testing.T) {
	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	base := signedV2PurchaseOrderRequest(t, srv)
	v1Input, err := delegationProofInput(base)
	if err != nil {
		t.Fatalf("build V1 downgrade input: %v", err)
	}
	v1Proof, err := srv.delegationMgr.GenerateProof(v1Input)
	if err != nil {
		t.Fatalf("generate V1 downgrade proof: %v", err)
	}

	cases := map[string]func(*policyv1.CheckAccessRequest){
		"missing proof": func(req *policyv1.CheckAccessRequest) {
			delete(req.Context, "delegation_proof")
		},
		"V1 downgrade": func(req *policyv1.CheckAccessRequest) {
			req.Context["delegation_proof"] = v1Proof
		},
		"unknown proof version": func(req *policyv1.CheckAccessRequest) {
			req.Context["delegation_proof"] = "v3.legacy." + strings.Repeat("0", 64)
		},
		"changed amount": func(req *policyv1.CheckAccessRequest) {
			req.Context["cbi.amount_minor"] = "123457"
		},
		"changed lines": func(req *policyv1.CheckAccessRequest) {
			req.Context["cbi.line_digest"] = strings.Repeat("2", 64)
		},
		"changed witness": func(req *policyv1.CheckAccessRequest) {
			req.Context["cbi.state_witness"] = strings.Repeat("0", 64)
		},
		"changed request action": func(req *policyv1.CheckAccessRequest) {
			req.Action = "action:APPROVE_PURCHASE_ORDER"
		},
		"unknown CBI field": func(req *policyv1.CheckAccessRequest) {
			req.Context["cbi.untrusted"] = "value"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := cloneCheckAccessRequest(base)
			mutate(req)
			if err := srv.validateDelegation(req); err == nil {
				t.Fatal("invalid high-impact request was accepted")
			}
		})
	}
}

func TestValidateDelegationKeepsExplicitLegacyV1Boundary(t *testing.T) {
	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	now := time.Now().Unix()
	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-legacy",
		Subject:  "agent:legacy",
		Action:   "action:APPROVE_PURCHASE_ORDER",
		Resource: "purchase_order:PO-LEGACY",
		Context: map[string]string{
			"amount":                 "1000",
			"delegation_grant_id":    "grant-legacy",
			"delegated_by":           "user:manager",
			"delegation_issued_at":   strconv.FormatInt(now-1, 10),
			"delegation_valid_until": strconv.FormatInt(now+600, 10),
			"delegation_nonce":       "legacy-command",
			"delegation_chain":       "user:manager,agent:legacy",
			"resource.creator_id":    "user:creator",
			"tool_context":           "tool:legacy",
			"execution_mode":         "legacy",
		},
	}
	input, err := delegationProofInput(req)
	if err != nil {
		t.Fatalf("build legacy input: %v", err)
	}
	req.Context["delegation_proof"], err = srv.delegationMgr.GenerateProof(input)
	if err != nil {
		t.Fatalf("sign legacy proof: %v", err)
	}
	if err := srv.validateDelegation(req); err != nil {
		t.Fatalf("explicit legacy V1 boundary was rejected: %v", err)
	}
}

func TestValidateDelegationDirectRequestDoesNotRequireCBI(t *testing.T) {
	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-direct",
		Subject:  "user:buyer",
		Action:   security.ConfirmPurchaseOrderAction,
		Resource: "purchase_order:42",
		Context:  map[string]string{},
	}
	if err := srv.validateDelegation(req); err != nil {
		t.Fatalf("direct request unexpectedly required CBI: %v", err)
	}
	if req.Context["delegation_chain"] != req.Subject {
		t.Fatal("direct request did not receive server-derived chain")
	}
}

func signedV2PurchaseOrderRequest(t *testing.T, srv *GRPCServer) *policyv1.CheckAccessRequest {
	t.Helper()
	now := time.Now().Unix()
	intent := security.CanonicalBusinessIntent{
		IntentVersion:      security.CanonicalBusinessIntentVersion,
		TenantID:           "tenant-v2",
		CompanyID:          7,
		ResourceType:       security.PurchaseOrderResourceType,
		ResourceID:         42,
		Action:             security.ConfirmPurchaseOrderAction,
		VendorID:           19,
		CurrencyCode:       "USD",
		CurrencyScale:      2,
		AmountMinor:        123456,
		LineDigest:         strings.Repeat("1", 64),
		RecordState:        "draft",
		RecordWriteVersion: "2026-09-20T01:02:03.456789Z",
		CreatorSubject:     "user:creator",
		DelegationGrantID:  9,
		DelegatorSubject:   "user:manager",
		AgentSubject:       "agent:procurement_copilot",
		CommandID:          base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
		ProofVersion:       security.CanonicalIntentProofVersion,
	}
	if err := intent.RefreshStateWitness(); err != nil {
		t.Fatalf("build CBI witness: %v", err)
	}
	input := security.DelegationProofV2Input{Intent: intent, IssuedAt: now - 1, ValidUntil: now + 600}
	proof, err := srv.delegationMgr.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("sign V2 proof: %v", err)
	}
	contextValues := map[string]string{
		"delegation_grant_id":    "9",
		"delegated_by":           intent.DelegatorSubject,
		"delegation_issued_at":   strconv.FormatInt(input.IssuedAt, 10),
		"delegation_valid_until": strconv.FormatInt(input.ValidUntil, 10),
		"delegation_nonce":       intent.CommandID,
		"delegation_chain":       intent.DelegatorSubject + "," + intent.AgentSubject,
		"delegation_proof":       proof,
		"resource.creator_id":    intent.CreatorSubject,
		"tool_context":           "tool:auto_confirm_po",
		"execution_mode":         "autonomous_run",
	}
	putCanonicalIntentContext(contextValues, intent)
	return &policyv1.CheckAccessRequest{
		TenantId: intent.TenantID,
		Subject:  intent.AgentSubject,
		Action:   intent.Action,
		Resource: "purchase_order:42",
		Context:  contextValues,
	}
}

func putCanonicalIntentContext(context map[string]string, intent security.CanonicalBusinessIntent) {
	values := map[string]string{
		"intent_version": intent.IntentVersion, "tenant_id": intent.TenantID,
		"company_id": strconv.FormatInt(intent.CompanyID, 10), "resource_type": intent.ResourceType,
		"resource_id": strconv.FormatInt(intent.ResourceID, 10), "action": intent.Action,
		"vendor_id": strconv.FormatInt(intent.VendorID, 10), "currency_code": intent.CurrencyCode,
		"currency_scale": strconv.FormatInt(intent.CurrencyScale, 10), "amount_minor": strconv.FormatInt(intent.AmountMinor, 10),
		"line_digest": intent.LineDigest, "record_state": intent.RecordState,
		"record_write_version": intent.RecordWriteVersion, "state_witness": intent.StateWitness,
		"creator_subject": intent.CreatorSubject, "delegation_grant_id": strconv.FormatInt(intent.DelegationGrantID, 10),
		"delegator_subject": intent.DelegatorSubject, "agent_subject": intent.AgentSubject,
		"command_id": intent.CommandID, "proof_version": intent.ProofVersion,
	}
	for field, value := range values {
		context[canonicalIntentContextPrefix+field] = value
	}
}

func cloneCheckAccessRequest(req *policyv1.CheckAccessRequest) *policyv1.CheckAccessRequest {
	return proto.Clone(req).(*policyv1.CheckAccessRequest)
}
