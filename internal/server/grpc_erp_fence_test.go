package server

import (
	"context"
	"errors"
	"testing"

	"standalone-policy-engine/internal/engine"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type scopedRevocationStore struct {
	stubRevocationStore
	failure error
}

func (*scopedRevocationStore) ERPRevocationFenceScope() string { return "odoo-revocation.v1:odoo" }

func (s *scopedRevocationStore) EnsureERPPolicyRevision(context.Context, string, uint64) error {
	return s.failure
}

func TestCheckAccessRequiresMatchingERPFence(t *testing.T) {
	configureGRPCTestJWT(t)
	srv := NewGRPCServer(engine.NewEngineWithGC(engine.GCConfig{Enabled: false}), nil)
	ctx := incomingJWTContext(t, "tenant-a", "user:manager")
	req := &policyv1.CheckAccessRequest{
		TenantId: "tenant-a", Action: "read", Resource: "document",
		Context: map[string]string{"erp.revocation_fence": "odoo-revocation.v1:odoo"},
	}
	if _, err := srv.CheckAccess(ctx, req); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unconfigured ERP fence must reject: %v", err)
	}
	srv.revocationStore = &scopedRevocationStore{}
	req.Context["erp.revocation_fence"] = "odoo-revocation.v1:other"
	if _, err := srv.CheckAccess(ctx, req); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("wrong ERP database must reject: %v", err)
	}
	req.Context["erp.revocation_fence"] = "odoo-revocation.v1:odoo"
	response, err := srv.CheckAccess(ctx, req)
	if err != nil || response.Advice["erp.revocation_fence"] != req.Context["erp.revocation_fence"] {
		t.Fatalf("configured scope must be acknowledged: response=%v err=%v", response, err)
	}
	if response.Advice["erp.policy_revision"] != "0" {
		t.Fatalf("missing evaluated revision: %v", response.Advice)
	}
	srv.revocationStore = &scopedRevocationStore{failure: errors.New("ERP unavailable")}
	req.Context = map[string]string{"erp.revocation_fence": "odoo-revocation.v1:odoo"}
	if _, err := srv.CheckAccess(ctx, req); status.Code(err) != codes.Unavailable {
		t.Fatalf("unavailable authority storage must fail closed: %v", err)
	}
}
