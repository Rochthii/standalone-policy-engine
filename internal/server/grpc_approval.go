package server

import (
	"context"
	"math"
	"strconv"
	"strings"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const approvalPolicyAction = "action:APPROVE_PURCHASE_ORDER"

func (s *GRPCServer) IssueApprovalCapability(ctx context.Context, req *policyv1.IssueApprovalCapabilityRequest) (*policyv1.IssueApprovalCapabilityResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.evaluationTimeout)
	defer cancel()
	if req == nil || s.approvalMgr == nil || s.engine == nil {
		return nil, status.Error(codes.FailedPrecondition, "approval authority is not configured")
	}
	claims, err := s.validateTenantAndGetClaims(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	approverSubject, trustedContext, err := s.bindTrustedIdentity(claims, req.Context)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid approval identity: %v", err)
	}
	if !strings.HasPrefix(approverSubject, "user:") || approverSubject == req.AgentSubject || approverSubject == req.CreatorSubject || approverSubject == req.DelegatorSubject {
		return nil, status.Error(codes.PermissionDenied, "approval identity violates separation of duties")
	}
	if trustedContext["principal.company_id"] != strconv.FormatInt(req.CompanyId, 10) {
		return nil, status.Error(codes.PermissionDenied, "approver company does not match signed identity")
	}
	if !strings.HasPrefix(req.Resource, "purchase_order:") || req.DelegationGrantId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid approval resource or delegation grant")
	}
	grantID := strconv.FormatInt(req.DelegationGrantId, 10)
	if s.delegationMgr == nil || !s.delegationMgr.RevocationReady() {
		return nil, status.Error(codes.Unavailable, "revocation state is not ready")
	}
	if s.delegationMgr.IsRevoked(req.TenantId, grantID) {
		return nil, status.Error(codes.PermissionDenied, "delegation grant is revoked")
	}

	trustedContext["approval_id"] = req.ApprovalId
	trustedContext["approval_state"] = "pending"
	trustedContext["intent_hash"] = req.IntentHash
	trustedContext["state_witness"] = req.StateWitness
	trustedContext["command_id"] = req.CommandId
	trustedContext["grant_id"] = grantID
	trustedContext["delegation_grant_id"] = grantID
	trustedContext["resource.creator_id"] = req.CreatorSubject
	result := s.engine.CheckPermission(ctx, req.TenantId, approverSubject, approvalPolicyAction, req.Resource, trustedContext)
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if result.Decision != engine.DecisionAllow || len(result.Obligations) != 0 {
		return nil, status.Error(codes.PermissionDenied, "current policy denied approval capability issuance")
	}
	revision := s.engine.GetTenantRevision(req.TenantId)
	if revision > math.MaxInt64 {
		return nil, status.Error(codes.Internal, "policy revision exceeds approval capability range")
	}
	capability, envelope, err := s.approvalMgr.Issue(security.ApprovalCapability{
		ApprovalID:             req.ApprovalId,
		TenantID:               req.TenantId,
		CompanyID:              req.CompanyId,
		IntentHash:             req.IntentHash,
		StateWitness:           req.StateWitness,
		CommandID:              req.CommandId,
		DelegationGrantID:      req.DelegationGrantId,
		DelegatorSubject:       req.DelegatorSubject,
		AgentSubject:           req.AgentSubject,
		CreatorSubject:         req.CreatorSubject,
		ApproverUserID:         req.ApproverUserId,
		ApproverSubject:        approverSubject,
		IssuancePolicyRevision: int64(revision),
	}, req.DelegationValidUntil)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid approval capability binding: %v", err)
	}
	return &policyv1.IssueApprovalCapabilityResponse{Capability: approvalCapabilityToProto(capability, envelope)}, nil
}

func (s *GRPCServer) VerifyApprovalCapability(ctx context.Context, req *policyv1.VerifyApprovalCapabilityRequest) (*policyv1.VerifyApprovalCapabilityResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.evaluationTimeout)
	defer cancel()
	if req == nil || req.Capability == nil || s.approvalMgr == nil {
		return nil, status.Error(codes.InvalidArgument, "approval capability is required")
	}
	claims, err := s.validateTenantAndGetClaims(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	approverSubject, trustedContext, err := s.bindTrustedIdentity(claims, nil)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid approval identity: %v", err)
	}
	capability := approvalCapabilityFromProto(req.Capability)
	if capability.TenantID != req.TenantId || capability.ApproverSubject != approverSubject || trustedContext["principal.company_id"] != strconv.FormatInt(capability.CompanyID, 10) {
		return nil, status.Error(codes.PermissionDenied, "approval capability identity binding mismatch")
	}
	envelope := security.ApprovalCapabilityEnvelope{
		Algorithm: req.Capability.Algorithm,
		KeyID:     req.Capability.KeyId,
		Payload:   req.Capability.CanonicalPayload,
		Signature: req.Capability.Signature,
	}
	if err := s.approvalMgr.Verify(capability, envelope); err != nil {
		return nil, status.Error(codes.PermissionDenied, "approval capability verification failed")
	}
	return &policyv1.VerifyApprovalCapabilityResponse{Valid: true}, nil
}

func approvalCapabilityToProto(capability security.ApprovalCapability, envelope security.ApprovalCapabilityEnvelope) *policyv1.ApprovalCapability {
	return &policyv1.ApprovalCapability{
		CapabilityVersion:      capability.CapabilityVersion,
		Purpose:                capability.Purpose,
		ApprovalId:             capability.ApprovalID,
		TenantId:               capability.TenantID,
		CompanyId:              capability.CompanyID,
		IntentHash:             capability.IntentHash,
		StateWitness:           capability.StateWitness,
		CommandId:              capability.CommandID,
		DelegationGrantId:      capability.DelegationGrantID,
		DelegatorSubject:       capability.DelegatorSubject,
		AgentSubject:           capability.AgentSubject,
		CreatorSubject:         capability.CreatorSubject,
		ApproverUserId:         capability.ApproverUserID,
		ApproverSubject:        capability.ApproverSubject,
		RequiredPermission:     capability.RequiredPermission,
		IssuancePolicyRevision: capability.IssuancePolicyRevision,
		IssuedAt:               capability.IssuedAt,
		ExpiresAt:              capability.ExpiresAt,
		OneTimeId:              capability.OneTimeID,
		Algorithm:              envelope.Algorithm,
		KeyId:                  envelope.KeyID,
		CanonicalPayload:       append([]byte(nil), envelope.Payload...),
		Signature:              append([]byte(nil), envelope.Signature...),
	}
}

func approvalCapabilityFromProto(source *policyv1.ApprovalCapability) security.ApprovalCapability {
	return security.ApprovalCapability{
		CapabilityVersion:      source.CapabilityVersion,
		Purpose:                source.Purpose,
		ApprovalID:             source.ApprovalId,
		TenantID:               source.TenantId,
		CompanyID:              source.CompanyId,
		IntentHash:             source.IntentHash,
		StateWitness:           source.StateWitness,
		CommandID:              source.CommandId,
		DelegationGrantID:      source.DelegationGrantId,
		DelegatorSubject:       source.DelegatorSubject,
		AgentSubject:           source.AgentSubject,
		CreatorSubject:         source.CreatorSubject,
		ApproverUserID:         source.ApproverUserId,
		ApproverSubject:        source.ApproverSubject,
		RequiredPermission:     source.RequiredPermission,
		IssuancePolicyRevision: source.IssuancePolicyRevision,
		IssuedAt:               source.IssuedAt,
		ExpiresAt:              source.ExpiresAt,
		OneTimeID:              source.OneTimeId,
	}
}
