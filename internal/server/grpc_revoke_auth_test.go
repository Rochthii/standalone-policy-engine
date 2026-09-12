package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"standalone-policy-engine/internal/engine"
	policyv1 "standalone-policy-engine/proto/v1"
)

func TestGRPCServer_RevokeDelegationAuthorization(t *testing.T) {
	configureGRPCTestJWT(t)
	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	request := func() *policyv1.RevokeRequest {
		return &policyv1.RevokeRequest{
			TenantId:  "tenant-a",
			GrantId:   "grant-a",
			RevokedBy: "user:manager",
			Reason:    "security response",
		}
	}

	tests := []struct {
		name string
		ctx  context.Context
		edit func(*policyv1.RevokeRequest)
		code codes.Code
	}{
		{name: "missing token", ctx: context.Background(), code: codes.Unauthenticated},
		{name: "missing permission", ctx: incomingJWTContext(t, "tenant-a", "user:manager"), code: codes.PermissionDenied},
		{
			name: "forged revoked_by",
			ctx:  incomingJWTContextWithPermissions(t, "tenant-a", "user:attacker", "delegation:revoke"),
			code: codes.PermissionDenied,
		},
		{
			name: "cross tenant",
			ctx:  incomingJWTContextWithPermissions(t, "tenant-b", "user:manager", "delegation:revoke"),
			code: codes.PermissionDenied,
		},
		{
			name: "authorized",
			ctx:  incomingJWTContextWithPermissions(t, "tenant-a", "user:manager", "delegation:revoke"),
			code: codes.OK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := request()
			if test.edit != nil {
				test.edit(req)
			}
			response, err := srv.RevokeDelegation(test.ctx, req)
			if got := status.Code(err); got != test.code {
				t.Fatalf("expected %s, got %s (%v)", test.code, got, err)
			}
			if test.code == codes.OK && (response == nil || !response.Success) {
				t.Fatalf("expected successful revoke response, got %#v", response)
			}
		})
	}
}
