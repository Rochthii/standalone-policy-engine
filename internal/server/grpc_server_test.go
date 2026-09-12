package server

import (
	"bytes"
	"context"
	"log"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	policyv1 "standalone-policy-engine/proto/v1"
)

func TestGRPCServer_TenantIsolation(t *testing.T) {
	configureGRPCTestJWT(t)
	secret := grpcTestJWTSecret

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

func TestGRPCServer_RequiredAuthentication(t *testing.T) {
	configureGRPCTestJWT(t)
	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	srv := NewGRPCServer(eng, nil)
	checkRequest := &policyv1.CheckAccessRequest{
		TenantId: "tenant-a",
		Subject:  "user:untrusted",
		Action:   "READ",
		Resource: "file:doc",
	}
	explainRequest := &policyv1.ExplainRequest{
		TenantId: "tenant-a",
		Subject:  "user:untrusted",
		Action:   "READ",
		Resource: "file:doc",
	}

	tests := []struct {
		name string
		ctx  context.Context
		call func(context.Context) error
	}{
		{
			name: "CheckAccess missing metadata",
			ctx:  context.Background(),
			call: func(ctx context.Context) error {
				_, err := srv.CheckAccess(ctx, checkRequest)
				return err
			},
		},
		{
			name: "ExplainDecision missing metadata",
			ctx:  context.Background(),
			call: func(ctx context.Context) error {
				_, err := srv.ExplainDecision(ctx, explainRequest)
				return err
			},
		},
		{
			name: "missing tenant claim",
			ctx: incomingJWTContextWithClaims(t, jwt.MapClaims{
				"sub": "user:test",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			call: func(ctx context.Context) error {
				_, err := srv.CheckAccess(ctx, checkRequest)
				return err
			},
		},
		{
			name: "missing subject claim",
			ctx: incomingJWTContextWithClaims(t, jwt.MapClaims{
				"tenant_id": "tenant-a",
				"exp":       time.Now().Add(time.Hour).Unix(),
			}),
			call: func(ctx context.Context) error {
				_, err := srv.CheckAccess(ctx, checkRequest)
				return err
			},
		},
		{
			name: "missing expiration claim",
			ctx: incomingJWTContextWithClaims(t, jwt.MapClaims{
				"sub":       "user:test",
				"tenant_id": "tenant-a",
			}),
			call: func(ctx context.Context) error {
				_, err := srv.CheckAccess(ctx, checkRequest)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call(test.ctx)
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("expected Unauthenticated, got %v", err)
			}
		})
	}
}

func TestGRPCServer_TrustedPrincipalAttributes(t *testing.T) {
	configureGRPCTestJWT(t)
	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	l := parser.NewLexer(`permit(principal == any, action == action:READ, resource == file:doc) when { principal.department == "finance" };`)
	p := parser.NewParser(l)
	nodes := p.Parse()
	if len(nodes) != 1 || len(p.Errors()) != 0 {
		t.Fatalf("parse trusted-attribute policy: %v", p.Errors())
	}
	nodes[0].ID = "policy-trusted-principal"
	compiled, err := parser.NewCompiler().Compile(nodes[0])
	if err != nil {
		t.Fatalf("compile trusted-attribute policy: %v", err)
	}
	if err := eng.UpdateTenantPoliciesWithRevision("tenant-a", []*parser.PolicyNode{compiled}, nil, 1); err != nil {
		t.Fatalf("load trusted-attribute policy: %v", err)
	}
	srv := NewGRPCServer(eng, nil)

	request := func(attributes map[string]string) *policyv1.CheckAccessRequest {
		return &policyv1.CheckAccessRequest{
			TenantId: "tenant-a",
			Subject:  "user:forged",
			Action:   "READ",
			Resource: "file:doc",
			Context:  attributes,
		}
	}
	claims := func(department string) jwt.MapClaims {
		result := jwt.MapClaims{
			"sub":       "user:alice",
			"tenant_id": "tenant-a",
			"exp":       time.Now().Add(time.Hour).Unix(),
		}
		if department != "" {
			result["department"] = department
		}
		return result
	}

	tests := []struct {
		name       string
		claims     jwt.MapClaims
		attributes map[string]string
		want       policyv1.CheckAccessResponse_Decision
	}{
		{
			name:       "token claim overrides forged namespaced attribute",
			claims:     claims("finance"),
			attributes: map[string]string{"principal.department": "engineering"},
			want:       policyv1.CheckAccessResponse_ALLOW,
		},
		{
			name:       "request cannot elevate a token claim",
			claims:     claims("engineering"),
			attributes: map[string]string{"principal.department": "finance"},
			want:       policyv1.CheckAccessResponse_DENY,
		},
		{
			name:       "raw fallback cannot invent missing principal claim",
			claims:     claims(""),
			attributes: map[string]string{"department": "finance"},
			want:       policyv1.CheckAccessResponse_DENY,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := incomingJWTContextWithClaims(t, test.claims)
			response, err := srv.CheckAccess(ctx, request(test.attributes))
			if err != nil {
				t.Fatalf("CheckAccess returned error: %v", err)
			}
			if response.Decision != test.want {
				t.Fatalf("expected %v, got %v", test.want, response.Decision)
			}
		})
	}
}

func TestGRPCServer_AuditLogWithRevision(t *testing.T) {
	configureGRPCTestJWT(t)
	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled: false,
	})

	// Cập nhật policy cho tenant-rev với Revision = 7
	l := parser.NewLexer(`permit(principal == role:admin, action == action:READ, resource == file:confidential);`)
	p := parser.NewParser(l)
	nodes := p.Parse()
	if len(nodes) == 0 {
		t.Fatal("Parse policy thất bại")
	}
	nodes[0].ID = "policy-audit-test"
	c := parser.NewCompiler()
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("Compile policy thất bại: %v", err)
	}
	_ = eng.UpdateTenantPoliciesWithRevision("tenant-rev", []*parser.PolicyNode{compiled}, nil, 7)

	buf := &bytes.Buffer{}
	logger := audit.NewStreamAuditLogger(buf)
	defer logger.Stop()

	srv := NewGRPCServer(eng, logger)

	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-rev",
		Subject:  "role:admin",
		Action:   "READ",
		Resource: "file:confidential",
	}

	resp, err := srv.CheckAccess(incomingJWTContext(t, "tenant-rev", "role:admin"), req)
	if err != nil {
		t.Fatalf("CheckAccess lỗi không mong muốn: %v", err)
	}
	if resp.Decision != policyv1.CheckAccessResponse_ALLOW {
		t.Errorf("Mong đợi ALLOW, nhận %v", resp.Decision)
	}

	if buf.Len() == 0 {
		t.Fatal("Buffer audit log không được rỗng")
	}

	entry, err := audit.DecodeNDJSONLogEntry(buf.Bytes())
	if err != nil {
		t.Fatalf("Decode audit log NDJSON lỗi: %v", err)
	}

	if entry.RevisionID != 7 {
		t.Errorf("RevisionID không khớp: mong đợi 7, nhận %d", entry.RevisionID)
	}
	if entry.TenantID != "tenant-rev" {
		t.Errorf("TenantID không khớp: %s", entry.TenantID)
	}
	if entry.Decision != "ALLOW" {
		t.Errorf("Decision không khớp: %s", entry.Decision)
	}
}
