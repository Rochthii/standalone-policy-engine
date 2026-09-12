package server

import (
	"context"
	"strings"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *GRPCServer) ExplainDecision(ctx context.Context, req *policyv1.ExplainRequest) (*policyv1.ExplainResponse, error) {
	claims, err := s.validateTenantAndGetClaims(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	subject, trustedContext, err := s.bindTrustedIdentity(claims, req.Context)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "JWT identity claims không hợp lệ: %v", err)
	}
	req.Subject = subject
	req.Context = trustedContext

	result := s.engine.CheckPermission(ctx, req.TenantId, req.Subject, req.Action, req.Resource, req.Context)
	trie, exists := s.engine.GetTenantTrie(ctx, req.TenantId)
	if !exists {
		return &policyv1.ExplainResponse{
			Decision:    policyv1.ExplainResponse_DENY,
			FinalReason: "Không tìm thấy tập chính sách cho Tenant",
			Matched:     []*policyv1.PolicyMetadata{},
		}, nil
	}

	subjects := trie.RoleDAG.GetInheritedRoles(req.Subject)
	action := req.Action
	if !strings.HasPrefix(action, "action:") {
		action = "action:" + action
	}
	matchedPolicies := trie.LookupPolicies(subjects, []string{req.Resource}, action)
	evalCtx := engine.GetEvalContext(req.Subject, req.Action, req.Resource, req.Context, trie.RoleDAG)
	defer evalCtx.Release()

	matchedMetadata := make([]*policyv1.PolicyMetadata, 0)
	for _, policy := range matchedPolicies {
		value, evalErr := engine.Evaluate(policy.Condition, evalCtx)
		conditionMatched := evalErr == nil && value.ValType == parser.ValueTypeBool && value.BoolVal
		if policy.IsUnless {
			conditionMatched = evalErr == nil && value.ValType == parser.ValueTypeBool && !value.BoolVal
		}
		if !conditionMatched {
			continue
		}
		effect := "permit"
		if policy.Effect == parser.EffectForbid {
			effect = "forbid"
		}
		matchedMetadata = append(matchedMetadata, &policyv1.PolicyMetadata{
			PolicyId:   policy.ID,
			Effect:     effect,
			PolicyText: policy.ID + ": " + effect + "(...) when { ... };",
		})
	}

	decision := policyv1.ExplainResponse_DENY
	if result.Decision == engine.DecisionAllow {
		decision = policyv1.ExplainResponse_ALLOW
	}
	return &policyv1.ExplainResponse{
		Decision:    decision,
		FinalReason: result.Reason,
		Matched:     matchedMetadata,
	}, nil
}
