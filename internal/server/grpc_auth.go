package server

import (
	"context"
	"log"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// validateTenantAndGetClaims authenticates a gRPC request and binds it to one tenant.
// Missing credentials or required identity claims fail closed before policy evaluation.
func (s *GRPCServer) validateTenantAndGetClaims(ctx context.Context, tenantID string) (jwt.MapClaims, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id không được để trống")
	}
	if s.jwtValidator == nil {
		return nil, status.Error(codes.Internal, "JWT validator chưa được cấu hình")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "thiếu metadata authorization")
	}
	authValues := md.Get("authorization")
	if len(authValues) != 1 {
		return nil, status.Error(codes.Unauthenticated, "yêu cầu đúng một Authorization header")
	}

	parts := strings.Fields(authValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return nil, status.Error(codes.Unauthenticated, "Authorization phải có dạng Bearer <token>")
	}

	claims, err := s.jwtValidator.ValidateToken(parts[1])
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "JWT token không hợp lệ: %v", err)
	}

	tokenTenantID, ok := claims["tenant_id"].(string)
	if !ok || strings.TrimSpace(tokenTenantID) == "" {
		return nil, status.Error(codes.Unauthenticated, "JWT thiếu tenant_id hợp lệ")
	}
	if tokenTenantID != tenantID {
		log.Printf("[Security] Cross-tenant request rejected: token.tenant_id=%s req.tenant_id=%s", tokenTenantID, tenantID)
		return nil, status.Error(codes.PermissionDenied, "tenant_id trong token không khớp với request")
	}

	subject, ok := claims["sub"].(string)
	if !ok || strings.TrimSpace(subject) == "" {
		return nil, status.Error(codes.Unauthenticated, "JWT thiếu sub hợp lệ")
	}

	return claims, nil
}

// bindTrustedIdentity replaces all client-supplied principal attributes with
// identity-provider claims. Resource and execution context remain request data
// until their own trusted source or proof binding is implemented.
func (s *GRPCServer) bindTrustedIdentity(claims jwt.MapClaims, requestContext map[string]string) (string, map[string]string, error) {
	subject, attributes, err := s.jwtValidator.ExtractSubjectAttributes(claims)
	if err != nil {
		return "", nil, err
	}

	return subject, mergeTrustedPrincipalContext(requestContext, attributes), nil
}
