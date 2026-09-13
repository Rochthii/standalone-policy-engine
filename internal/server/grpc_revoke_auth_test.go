package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
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

type stubRevocationStore struct {
	record security.RevocationRecord
	err    error
}

func (s *stubRevocationStore) PersistRevocation(_ context.Context, record security.RevocationRecord) (security.RevocationRecord, error) {
	if s.err != nil {
		return security.RevocationRecord{}, s.err
	}
	if !s.record.RevokedAt.IsZero() {
		return s.record, nil
	}
	return record, nil
}

func (*stubRevocationStore) WatchRevocations(context.Context, func([]security.RevocationRecord), func(security.RevocationRecord)) error {
	return nil
}

func TestGRPCServer_RevokeDelegationRequiresDurableCommit(t *testing.T) {
	configureGRPCTestJWT(t)
	ctx := incomingJWTContextWithPermissions(t, "tenant-a", "user:manager", "delegation:revoke")
	req := &policyv1.RevokeRequest{TenantId: "tenant-a", GrantId: "grant-a", RevokedBy: "user:manager"}

	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	srv.revocationStore = &stubRevocationStore{err: errors.New("database unavailable")}
	if _, err := srv.RevokeDelegation(ctx, req); status.Code(err) != codes.Unavailable {
		t.Fatalf("durable failure must fail closed with Unavailable, got %v", err)
	}
	if srv.delegationMgr.IsRevoked(req.TenantId, req.GrantId) {
		t.Fatal("failed durable commit must not report a process-local-only revoke")
	}

	now := time.Now().UTC()
	srv.revocationStore = &stubRevocationStore{record: security.RevocationRecord{
		TenantID: req.TenantId, GrantID: req.GrantId, RevokedBy: req.RevokedBy,
		RevokedAt: now, ExpiresAt: now.Add(time.Hour),
	}}
	response, err := srv.RevokeDelegation(ctx, req)
	if err != nil || response.RevokedAt != now.Unix() || !srv.delegationMgr.IsRevoked(req.TenantId, req.GrantId) {
		t.Fatalf("durable revoke was not installed locally: response=%#v err=%v", response, err)
	}
}
