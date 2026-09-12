package server

import (
	"context"
	"fmt"
	"log"

	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *GRPCServer) RevokeDelegation(ctx context.Context, req *policyv1.RevokeRequest) (*policyv1.RevokeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "revoke request không được để trống")
	}
	claims, err := s.validateTenantAndGetClaims(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	subject, _, err := s.jwtValidator.ExtractSubjectAttributes(claims)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "JWT identity claims không hợp lệ: %v", err)
	}
	if req.RevokedBy != "" && req.RevokedBy != subject {
		return nil, status.Error(codes.PermissionDenied, "revoked_by không khớp với JWT subject")
	}
	if !security.HasPermission(claims, "delegation:revoke") {
		return nil, status.Error(codes.PermissionDenied, "JWT thiếu quyền delegation:revoke")
	}
	if req.GrantId == "" {
		return nil, status.Error(codes.InvalidArgument, "grant_id không được để trống")
	}
	if s.delegationMgr == nil {
		return nil, status.Error(codes.Internal, "delegation manager chưa được cấu hình")
	}

	revokedAt := s.delegationMgr.Revoke(req.TenantId, req.GrantId)
	log.Printf("[Security] Delegation grant %s revoked by %s (tenant=%s)", req.GrantId, subject, req.TenantId)
	return &policyv1.RevokeResponse{
		Success:   true,
		RevokedAt: revokedAt,
		Message:   fmt.Sprintf("Thu hồi thành công phiên ủy quyền %s", req.GrantId),
	}, nil
}
