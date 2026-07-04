package server

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"standalone-policy-engine/internal/engine"
	policyv1 "standalone-policy-engine/proto/v1"
)

func TestGRPCServer_TenantIsolation(t *testing.T) {
	// Su dung cung secret key voi middleware_test.go
	secret := "test-secret-key-for-sprint-6-unit-test"

	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled:     true,
		Interval:    1 * time.Hour,
		IdleTimeout: 1 * time.Hour,
	})
	srv := NewGRPCServer(eng, nil)

	// Helper tao token
	makeToken := func(tenantID string) string {
		claims := jwt.MapClaims{
			"sub":       "user:test",
			"tenant_id": tenantID,
			"exp":       time.Now().Add(1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte(secret))
		return tokenStr
	}

	t.Run("Mismatched Tenant ID", func(t *testing.T) {
		token := makeToken("tenant-a")
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-b", // Mismatch
			Subject:  "user:test",
			Action:   "READ",
			Resource: "file:doc",
		}

		_, err := srv.CheckAccess(ctx, req)
		if err == nil {
			t.Fatal("Mong doi loi nhung thuc te thanh cong")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Loi thuc te khong phai status error: %v", err)
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("Mong doi PermissionDenied, thuc te %s", st.Code())
		}
	})

	t.Run("Matched Tenant ID", func(t *testing.T) {
		token := makeToken("tenant-a")
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-a", // Match
			Subject:  "user:test",
			Action:   "READ",
			Resource: "file:doc",
		}

		// Tra ve DENY mac dinh nhung khong duoc bao loi Auth
		resp, err := srv.CheckAccess(ctx, req)
		if err != nil {
			t.Fatalf("CheckAccess gap loi khong mong muon: %v", err)
		}
		if resp.Decision != policyv1.CheckAccessResponse_DENY {
			t.Errorf("Mong doi DENY (default), thuc te %v", resp.Decision)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		// Token ky bang secret khac
		badClaims := jwt.MapClaims{
			"sub":       "user:test",
			"tenant_id": "tenant-a",
			"exp":       time.Now().Add(1 * time.Hour).Unix(),
		}
		badToken := jwt.NewWithClaims(jwt.SigningMethodHS256, badClaims)
		badTokenStr, _ := badToken.SignedString([]byte("wrong-secret"))

		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+badTokenStr))

		req := &policyv1.CheckAccessRequest{
			TenantId: "tenant-a",
			Subject:  "user:test",
			Action:   "READ",
			Resource: "file:doc",
		}

		_, err := srv.CheckAccess(ctx, req)
		if err == nil {
			t.Fatal("Mong doi loi nhung thuc te thanh cong")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Loi thuc te khong phai status error: %v", err)
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Mong doi Unauthenticated, thuc te %s", st.Code())
		}
		log.Printf("Unauthenticated error test passed: %v", err)
	})

	t.Run("Mismatched Tenant ID ExplainDecision", func(t *testing.T) {
		token := makeToken("tenant-a")
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

		req := &policyv1.ExplainRequest{
			TenantId: "tenant-b", // Mismatch
			Subject:  "user:test",
			Action:   "READ",
			Resource: "file:doc",
		}

		_, err := srv.ExplainDecision(ctx, req)
		if err == nil {
			t.Fatal("Mong doi loi nhung thuc te thanh cong")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Loi thuc te khong phai status error: %v", err)
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("Mong doi PermissionDenied, thuc te %s", st.Code())
		}
	})

	t.Run("Matched Tenant ID ExplainDecision", func(t *testing.T) {
		token := makeToken("tenant-a")
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

		req := &policyv1.ExplainRequest{
			TenantId: "tenant-a", // Match
			Subject:  "user:test",
			Action:   "READ",
			Resource: "file:doc",
		}

		// Tra ve phan hoi loi "Không tìm thấy tập chính sách cho Tenant" nhung khong phai loi Auth
		resp, err := srv.ExplainDecision(ctx, req)
		if err != nil {
			t.Fatalf("ExplainDecision gap loi khong mong muon: %v", err)
		}
		if resp.Decision != policyv1.ExplainResponse_DENY {
			t.Errorf("Mong doi DENY (default), thuc te %v", resp.Decision)
		}
	})
}
