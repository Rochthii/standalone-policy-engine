package server

import (
	"fmt"
	"strconv"
	"strings"

	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var delegationContextKeys = []string{
	"delegation_proof",
	"delegated_by",
	"delegation_valid_until",
	"delegation_issued_at",
	"delegation_nonce",
	"delegation_chain",
}

func (s *GRPCServer) validateDelegation(req *policyv1.CheckAccessRequest) error {
	grantID := req.Context["delegation_grant_id"]
	if grantID == "" {
		for _, key := range delegationContextKeys {
			if req.Context[key] != "" {
				return status.Errorf(codes.PermissionDenied, "%s không hợp lệ khi thiếu delegation_grant_id", key)
			}
		}
		// A direct request gets a server-derived chain, never a caller-supplied one.
		req.Context["delegation_chain"] = req.Subject
		return nil
	}
	if s.delegationMgr == nil {
		return status.Error(codes.Internal, "delegation manager chưa được cấu hình")
	}

	input, err := delegationProofInput(req)
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "delegation tuple không hợp lệ: %v", err)
	}
	proof := req.Context["delegation_proof"]
	if proof == "" {
		return status.Error(codes.PermissionDenied, "delegated request bắt buộc có delegation_proof")
	}
	if err := s.delegationMgr.VerifyProof(input, proof); err != nil {
		return status.Error(codes.PermissionDenied, "delegation_proof không hợp lệ hoặc đã hết hạn")
	}
	return nil
}

func delegationProofInput(req *policyv1.CheckAccessRequest) (security.DelegationProofInput, error) {
	issuedAt, err := parseDelegationTimestamp(req.Context["delegation_issued_at"], "delegation_issued_at")
	if err != nil {
		return security.DelegationProofInput{}, err
	}
	validUntil, err := parseDelegationTimestamp(req.Context["delegation_valid_until"], "delegation_valid_until")
	if err != nil {
		return security.DelegationProofInput{}, err
	}
	return security.DelegationProofInput{
		TenantID:        req.TenantId,
		GrantID:         req.Context["delegation_grant_id"],
		Delegator:       req.Context["delegated_by"],
		Agent:           req.Subject,
		Action:          req.Action,
		Resource:        req.Resource,
		Amount:          req.Context["amount"],
		DelegationChain: req.Context["delegation_chain"],
		CreatorID:       req.Context["resource.creator_id"],
		ToolContext:     req.Context["tool_context"],
		ExecutionMode:   req.Context["execution_mode"],
		Nonce:           req.Context["delegation_nonce"],
		IssuedAt:        issuedAt,
		ValidUntil:      validUntil,
	}, nil
}

func parseDelegationTimestamp(value, field string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("missing %s", field)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}
