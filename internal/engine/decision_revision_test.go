package engine

import (
	"context"
	"standalone-policy-engine/internal/parser"
	"testing"
)

func TestDecisionCarriesEvaluatedSnapshotRevision(t *testing.T) {
	eng := NewEngineWithGC(GCConfig{Enabled: false})
	policy := compileTestPolicy(t, "permit", `permit(principal == any, action == any, resource == any);`)
	eng.SetLazyLoader(func(ctx context.Context, tenant string) error {
		return eng.UpdateTenantPoliciesWithRevision(tenant, []*parser.PolicyNode{policy}, nil, 41)
	})
	result := eng.CheckPermission(context.Background(), "tenant", "user:a", "read", "document", nil)
	if result.Decision != DecisionAllow || result.PolicyRevision != 41 {
		t.Fatalf("wrong lazy snapshot: %+v", result)
	}
	if err := eng.UpdateTenantPoliciesWithRevision("tenant", nil, nil, 42); err != nil {
		t.Fatal(err)
	}
	if result.PolicyRevision != 41 {
		t.Fatal("published decision revision changed")
	}
	result = eng.CheckPermission(context.Background(), "tenant", "user:a", "read", "document", nil)
	if result.Decision != DecisionDeny || result.PolicyRevision != 42 {
		t.Fatalf("wrong next snapshot: %+v", result)
	}
}
