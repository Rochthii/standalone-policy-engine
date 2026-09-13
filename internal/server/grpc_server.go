package server

import (
	"context"
	"log"
	"time"

	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	policyv1.UnimplementedPolicyDecisionPointServer

	engine            *engine.EngineWithGC
	auditLogger       *audit.AuditLogger
	jwtValidator      *security.JWTValidator
	delegationMgr     *security.DelegationManager
	revocationStore   security.RevocationStore
	evaluationTimeout time.Duration
}

const defaultEvaluationTimeout = 100 * time.Millisecond

func NewGRPCServer(eng *engine.EngineWithGC, logger *audit.AuditLogger) *GRPCServer {
	return &GRPCServer{
		engine:            eng,
		auditLogger:       logger,
		jwtValidator:      security.NewJWTValidator(),
		delegationMgr:     security.NewDelegationManager(),
		evaluationTimeout: defaultEvaluationTimeout,
	}
}

func NewGRPCServerWithSecurity(
	eng *engine.EngineWithGC,
	logger *audit.AuditLogger,
	securityConfig config.SecurityConfig,
	serverConfig config.ServerConfig,
) (*GRPCServer, error) {
	return newGRPCServerWithSecurity(eng, logger, securityConfig, serverConfig, nil)
}

func newGRPCServerWithSecurity(
	eng *engine.EngineWithGC,
	logger *audit.AuditLogger,
	securityConfig config.SecurityConfig,
	serverConfig config.ServerConfig,
	revocationStore security.RevocationStore,
) (*GRPCServer, error) {
	activeKeyID := securityConfig.DelegationActiveKeyID
	delegationKeys := securityConfig.DelegationKeys
	if len(delegationKeys) == 0 && securityConfig.DelegationSecret != "" {
		activeKeyID = "legacy"
		delegationKeys = map[string]string{"legacy": securityConfig.DelegationSecret}
	}
	delegationManager, err := security.NewDelegationManagerWithKeyring(
		activeKeyID,
		delegationKeys,
	)
	if err != nil {
		return nil, err
	}
	return &GRPCServer{
		engine:      eng,
		auditLogger: logger,
		jwtValidator: security.NewJWTValidatorWithConfig(
			securityConfig.JWTSecret,
			securityConfig.JWTIssuer,
			securityConfig.JWTAudience,
		),
		delegationMgr:     delegationManager,
		revocationStore:   revocationStore,
		evaluationTimeout: serverConfig.EvaluationTimeout,
	}, nil
}

func (s *GRPCServer) CheckAccess(ctx context.Context, req *policyv1.CheckAccessRequest) (*policyv1.CheckAccessResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.evaluationTimeout)
	defer cancel()
	startTime := time.Now()

	claims, err := s.validateTenantAndGetClaims(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	sub, trustedContext, err := s.bindTrustedIdentity(claims, req.Context)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "JWT identity claims không hợp lệ: %v", err)
	}
	req.Subject = sub
	req.Context = trustedContext

	if grantID := req.Context["delegation_grant_id"]; grantID != "" {
		if s.delegationMgr != nil && !s.delegationMgr.RevocationReady() {
			return nil, status.Error(codes.Unavailable, "revocation state chưa sẵn sàng")
		}
		if s.delegationMgr != nil && s.delegationMgr.IsRevoked(req.TenantId, grantID) {
			log.Printf("[Security] TOCTOU Violation: Delegation grant %s is revoked", grantID)
			return revokedDecision(), nil
		}
	}
	if err := s.validateDelegation(req); err != nil {
		log.Printf("[Security] Delegation validation failed for grant %s", req.Context["delegation_grant_id"])
		return nil, err
	}

	result := s.engine.CheckPermission(ctx, req.TenantId, req.Subject, req.Action, req.Resource, req.Context)
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return nil, status.Errorf(codes.DeadlineExceeded, "deadline exceeded: %v", err)
		}
		return nil, status.Errorf(codes.Canceled, "request canceled: %v", err)
	}
	decision := result.Decision.String()
	metrics.ObserveEvaluationDuration(req.TenantId, decision, time.Since(startTime))
	metrics.IncrementRequestCounter(req.TenantId, decision)

	matchedPolicyID := ""
	if len(result.Explanations) > 0 {
		matchedPolicyID = result.Explanations[0]
	}
	if s.auditLogger != nil {
		s.auditLogger.Log(
			s.engine.GetTenantRevision(req.TenantId),
			req.TenantId,
			req.Subject,
			req.Action,
			req.Resource,
			decision,
			matchedPolicyID,
			withAuditCorrelation(ctx, req.Context),
		)
	}

	decisionValue := policyv1.CheckAccessResponse_DENY
	if result.Decision == engine.DecisionAllow {
		decisionValue = policyv1.CheckAccessResponse_ALLOW
	}
	return &policyv1.CheckAccessResponse{
		Decision:        decisionValue,
		MatchedPolicyId: matchedPolicyID,
		Obligations:     protocolObligations(result.Obligations),
	}, nil
}

func protocolObligations(source []engine.Obligation) []*policyv1.Obligation {
	if len(source) == 0 {
		return nil
	}
	result := make([]*policyv1.Obligation, len(source))
	for i, obligation := range source {
		result[i] = &policyv1.Obligation{
			Type:    string(obligation.Type),
			Message: obligation.Message,
			Payload: obligation.Payload,
		}
	}
	return result
}

func revokedDecision() *policyv1.CheckAccessResponse {
	return &policyv1.CheckAccessResponse{
		Decision:        policyv1.CheckAccessResponse_DENY,
		MatchedPolicyId: "POL-REVOCATION-BLACK-LIST",
		Advice: map[string]string{
			"reason": "Phiên ủy quyền đã bị thu hồi bởi người giám sát (Revoked Delegation)",
		},
	}
}
