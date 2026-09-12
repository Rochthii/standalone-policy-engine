package server

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	policyv1 "standalone-policy-engine/proto/v1"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestGRPCServerRejectsOversizedProtobufRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	securityConfig := config.SecurityConfig{
		JWTSecret:        grpcTestJWTSecret,
		JWTIssuer:        "grpc-runtime-test",
		JWTAudience:      "policy-engine-test",
		DelegationSecret: "delegation-secret-for-grpc-runtime-test",
	}
	serverConfig := config.ServerConfig{
		EvaluationTimeout:   100 * time.Millisecond,
		GRPCMaxReceiveBytes: 1024,
		GRPCMaxSendBytes:    1024 * 1024,
	}
	grpcServer, err := StartGRPCServer(
		listener,
		engine.NewEngineWithGC(engine.GCConfig{Enabled: false}),
		nil,
		securityConfig,
		serverConfig,
	)
	if err != nil {
		t.Fatalf("start gRPC server: %v", err)
	}
	t.Cleanup(grpcServer.Stop)

	connection, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       "user:alice",
		"tenant_id": "tenant-a",
		"iss":       securityConfig.JWTIssuer,
		"aud":       securityConfig.JWTAudience,
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(securityConfig.JWTSecret))
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenString))
	_, err = policyv1.NewPolicyDecisionPointClient(connection).CheckAccess(ctx, &policyv1.CheckAccessRequest{
		TenantId: "tenant-a",
		Subject:  "user:alice",
		Action:   "READ",
		Resource: "file:" + strings.Repeat("x", 2048),
	})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted for oversized protobuf request, got %v", err)
	}
}
