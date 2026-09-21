package tests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"standalone-policy-engine/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
)

const integrationTestJWTSecret = "test-secret-key-for-in-process-grpc-tests"

func configureIntegrationTestJWT(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", integrationTestJWTSecret)
}

func signedDelegationContext(
	t *testing.T,
	mgr *security.DelegationManager,
	tenantID, grantID, delegator, agent, action, resource, amount, validUntil, creatorID string,
) map[string]string {
	t.Helper()
	expiresAt, err := strconv.ParseInt(validUntil, 10, 64)
	if err != nil {
		t.Fatalf("parse test delegation expiration: %v", err)
	}
	issuedAt := time.Now().Unix()
	if expiresAt <= issuedAt {
		issuedAt = expiresAt - int64(time.Hour/time.Second)
	}
	if action == security.ConfirmPurchaseOrderAction {
		return signedDelegationContextV2(t, mgr, tenantID, grantID, delegator, agent, resource, amount, creatorID, issuedAt, expiresAt)
	}
	contextValues := map[string]string{
		"delegation_grant_id":    grantID,
		"delegated_by":           delegator,
		"amount":                 amount,
		"delegation_issued_at":   strconv.FormatInt(issuedAt, 10),
		"delegation_valid_until": validUntil,
		"delegation_nonce":       grantID + "-nonce",
		"delegation_chain":       delegator + "," + agent,
		"resource.creator_id":    creatorID,
		"tool_context":           "tool:auto_confirm_po",
		"execution_mode":         "autonomous_run",
	}
	input := security.DelegationProofInput{
		TenantID:        tenantID,
		GrantID:         grantID,
		Delegator:       delegator,
		Agent:           agent,
		Action:          action,
		Resource:        resource,
		Amount:          amount,
		DelegationChain: contextValues["delegation_chain"],
		CreatorID:       creatorID,
		ToolContext:     contextValues["tool_context"],
		ExecutionMode:   contextValues["execution_mode"],
		Nonce:           contextValues["delegation_nonce"],
		IssuedAt:        issuedAt,
		ValidUntil:      expiresAt,
	}
	proof, err := mgr.GenerateProof(input)
	if err != nil {
		t.Fatalf("generate test delegation proof: %v", err)
	}
	contextValues["delegation_proof"] = proof
	return contextValues
}

func signedDelegationContextV2(
	t *testing.T,
	mgr *security.DelegationManager,
	tenantID, grantID, delegator, agent, resource, amount, creatorID string,
	issuedAt, expiresAt int64,
) map[string]string {
	t.Helper()
	parsedGrantID, err := strconv.ParseInt(grantID, 10, 64)
	if err != nil || parsedGrantID <= 0 {
		t.Fatalf("parse V2 test grant ID: %q", grantID)
	}
	resourceID, err := strconv.ParseInt(strings.TrimPrefix(resource, "purchase_order:"), 10, 64)
	if err != nil || resourceID <= 0 {
		t.Fatalf("parse V2 test resource ID: %q", resource)
	}
	amountMinor, err := strconv.ParseInt(amount, 10, 64)
	if err != nil || amountMinor < 0 {
		t.Fatalf("parse V2 test amount: %q", amount)
	}
	commandDigest := sha256.Sum256([]byte("command:" + grantID + ":" + resource))
	lineDigest := sha256.Sum256([]byte("lines:" + resource))
	intent := security.CanonicalBusinessIntent{
		IntentVersion:      security.CanonicalBusinessIntentVersion,
		TenantID:           tenantID,
		CompanyID:          1,
		ResourceType:       security.PurchaseOrderResourceType,
		ResourceID:         resourceID,
		Action:             security.ConfirmPurchaseOrderAction,
		VendorID:           1,
		CurrencyCode:       "USD",
		CurrencyScale:      0,
		AmountMinor:        amountMinor,
		LineDigest:         hex.EncodeToString(lineDigest[:]),
		RecordState:        "draft",
		RecordWriteVersion: "2026-09-21T00:00:00.000000Z",
		CreatorSubject:     creatorID,
		DelegationGrantID:  parsedGrantID,
		DelegatorSubject:   delegator,
		AgentSubject:       agent,
		CommandID:          base64.RawURLEncoding.EncodeToString(commandDigest[:]),
		ProofVersion:       security.CanonicalIntentProofVersion,
	}
	if err := intent.RefreshStateWitness(); err != nil {
		t.Fatalf("build V2 test intent: %v", err)
	}
	input := security.DelegationProofV2Input{Intent: intent, IssuedAt: issuedAt, ValidUntil: expiresAt}
	proof, err := mgr.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("generate V2 test delegation proof: %v", err)
	}
	contextValues := map[string]string{
		"delegation_grant_id":    grantID,
		"delegated_by":           delegator,
		"delegation_issued_at":   strconv.FormatInt(issuedAt, 10),
		"delegation_valid_until": strconv.FormatInt(expiresAt, 10),
		"delegation_nonce":       intent.CommandID,
		"delegation_chain":       delegator + "," + agent,
		"delegation_proof":       proof,
		"resource.creator_id":    creatorID,
		"tool_context":           "tool:auto_confirm_po",
		"execution_mode":         "autonomous_run",
	}
	cbiValues := map[string]string{
		"intent_version": intent.IntentVersion, "tenant_id": intent.TenantID,
		"company_id": strconv.FormatInt(intent.CompanyID, 10), "resource_type": intent.ResourceType,
		"resource_id": strconv.FormatInt(intent.ResourceID, 10), "action": intent.Action,
		"vendor_id": strconv.FormatInt(intent.VendorID, 10), "currency_code": intent.CurrencyCode,
		"currency_scale": strconv.FormatInt(intent.CurrencyScale, 10), "amount_minor": strconv.FormatInt(intent.AmountMinor, 10),
		"line_digest": intent.LineDigest, "record_state": intent.RecordState,
		"record_write_version": intent.RecordWriteVersion, "state_witness": intent.StateWitness,
		"creator_subject": intent.CreatorSubject, "delegation_grant_id": grantID,
		"delegator_subject": intent.DelegatorSubject, "agent_subject": intent.AgentSubject,
		"command_id": intent.CommandID, "proof_version": intent.ProofVersion,
	}
	for field, value := range cbiValues {
		contextValues["cbi."+field] = value
	}
	return contextValues
}

func authenticatedIncomingContext(t *testing.T, tenantID, subject string) context.Context {
	t.Helper()
	return authenticatedIncomingContextWithPermissions(t, tenantID, subject)
}

func authenticatedIncomingContextWithPermissions(t *testing.T, tenantID, subject string, permissions ...string) context.Context {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":         subject,
		"tenant_id":   tenantID,
		"permissions": permissions,
		"exp":         time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(integrationTestJWTSecret))
	if err != nil {
		t.Fatalf("sign integration test JWT: %v", err)
	}
	return metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+tokenString),
	)
}
