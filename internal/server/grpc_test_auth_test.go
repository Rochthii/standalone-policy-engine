package server

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
)

const grpcTestJWTSecret = "test-secret-key-for-grpc-security-tests"

func configureGRPCTestJWT(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", grpcTestJWTSecret)
}

func incomingJWTContext(t *testing.T, tenantID, subject string) context.Context {
	t.Helper()
	return incomingJWTContextWithClaims(t, jwt.MapClaims{
		"sub":       subject,
		"tenant_id": tenantID,
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
}

func incomingJWTContextWithPermissions(t *testing.T, tenantID, subject string, permissions ...string) context.Context {
	t.Helper()
	return incomingJWTContextWithClaims(t, jwt.MapClaims{
		"sub":         subject,
		"tenant_id":   tenantID,
		"permissions": permissions,
		"exp":         time.Now().Add(time.Hour).Unix(),
	})
}

func incomingJWTContextWithClaims(t *testing.T, claims jwt.MapClaims) context.Context {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(grpcTestJWTSecret))
	if err != nil {
		t.Fatalf("sign test JWT: %v", err)
	}
	return metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+tokenString),
	)
}
