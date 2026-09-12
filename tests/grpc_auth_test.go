package tests

import (
	"context"
	"strconv"
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
