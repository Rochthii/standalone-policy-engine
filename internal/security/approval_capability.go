package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
)

const (
	ApprovalCapabilityVersion   = "ac.v1"
	ApprovalCapabilityPurpose   = "odoo.purchase_order.confirm"
	ApprovalRequiredPermission  = "approval:purchase_order.confirm"
	ApprovalCapabilityAlgorithm = "HS256"
	approvalCapabilityDomain    = "PDP-APPROVAL-CAPABILITY-V1"
	MaxApprovalCapabilityTTL    = 24 * time.Hour
	approvalCapabilityClockSkew = 30 * time.Second
)

var approvalKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type ApprovalCapability struct {
	CapabilityVersion      string
	Purpose                string
	ApprovalID             string
	TenantID               string
	CompanyID              int64
	IntentHash             string
	StateWitness           string
	CommandID              string
	DelegationGrantID      int64
	DelegatorSubject       string
	AgentSubject           string
	CreatorSubject         string
	ApproverUserID         int64
	ApproverSubject        string
	RequiredPermission     string
	IssuancePolicyRevision int64
	IssuedAt               int64
	ExpiresAt              int64
	OneTimeID              string
}

type ApprovalCapabilityEnvelope struct {
	Algorithm string
	KeyID     string
	Payload   []byte
	Signature []byte
}

type ApprovalCapabilityManager struct {
	activeKeyID string
	keys        map[string][]byte
	ttl         time.Duration
	now         func() time.Time
	random      io.Reader
}

func NewApprovalCapabilityManagerWithKeyring(activeKeyID string, keys map[string]string, ttl time.Duration) (*ApprovalCapabilityManager, error) {
	if !approvalKeyIDPattern.MatchString(activeKeyID) {
		return nil, errors.New("invalid active approval key id")
	}
	if ttl <= 0 || ttl > MaxApprovalCapabilityTTL {
		return nil, fmt.Errorf("approval capability TTL must be within (0, %s]", MaxApprovalCapabilityTTL)
	}
	copied := make(map[string][]byte, len(keys))
	for keyID, secret := range keys {
		if !approvalKeyIDPattern.MatchString(keyID) || len(secret) < 32 {
			return nil, fmt.Errorf("invalid approval key %q", keyID)
		}
		copied[keyID] = append([]byte(nil), []byte(secret)...)
	}
	if _, ok := copied[activeKeyID]; !ok {
		return nil, errors.New("active approval key is absent from key ring")
	}
	return &ApprovalCapabilityManager{
		activeKeyID: activeKeyID,
		keys:        copied,
		ttl:         ttl,
		now:         time.Now,
		random:      rand.Reader,
	}, nil
}

func (m *ApprovalCapabilityManager) Issue(capability ApprovalCapability, delegationValidUntil int64) (ApprovalCapability, ApprovalCapabilityEnvelope, error) {
	if m == nil {
		return ApprovalCapability{}, ApprovalCapabilityEnvelope{}, errors.New("approval capability signer is not configured")
	}
	now := m.now().UTC().Unix()
	expiresAt := now + int64(m.ttl/time.Second)
	if delegationValidUntil < expiresAt {
		expiresAt = delegationValidUntil
	}
	if expiresAt <= now {
		return ApprovalCapability{}, ApprovalCapabilityEnvelope{}, errors.New("delegation is expired at approval issuance")
	}
	oneTimeID, err := randomBase64URL(m.random, 32)
	if err != nil {
		return ApprovalCapability{}, ApprovalCapabilityEnvelope{}, fmt.Errorf("generate approval one-time id: %w", err)
	}
	capability.CapabilityVersion = ApprovalCapabilityVersion
	capability.Purpose = ApprovalCapabilityPurpose
	capability.RequiredPermission = ApprovalRequiredPermission
	capability.IssuedAt = now
	capability.ExpiresAt = expiresAt
	capability.OneTimeID = oneTimeID
	payload, err := capability.CanonicalBytes()
	if err != nil {
		return ApprovalCapability{}, ApprovalCapabilityEnvelope{}, err
	}
	secret := m.keys[m.activeKeyID]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return capability, ApprovalCapabilityEnvelope{
		Algorithm: ApprovalCapabilityAlgorithm,
		KeyID:     m.activeKeyID,
		Payload:   payload,
		Signature: mac.Sum(nil),
	}, nil
}

func (m *ApprovalCapabilityManager) Verify(capability ApprovalCapability, envelope ApprovalCapabilityEnvelope) error {
	if m == nil || len(m.keys) == 0 {
		return errors.New("approval capability verifier is not configured")
	}
	if envelope.Algorithm != ApprovalCapabilityAlgorithm || !approvalKeyIDPattern.MatchString(envelope.KeyID) {
		return errors.New("unsupported approval capability algorithm or key id")
	}
	secret, ok := m.keys[envelope.KeyID]
	if !ok {
		return errors.New("unknown approval capability key")
	}
	expectedPayload, err := capability.CanonicalBytes()
	if err != nil {
		return err
	}
	if !bytes.Equal(expectedPayload, envelope.Payload) {
		return errors.New("approval capability payload binding mismatch")
	}
	if len(envelope.Signature) != sha256.Size {
		return errors.New("malformed approval capability signature")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(envelope.Payload)
	if !hmac.Equal(mac.Sum(nil), envelope.Signature) {
		return errors.New("approval capability signature mismatch")
	}
	now := m.now().UTC()
	if capability.IssuedAt > now.Add(approvalCapabilityClockSkew).Unix() {
		return errors.New("approval capability is not active yet")
	}
	if now.Unix() > capability.ExpiresAt {
		return errors.New("approval capability has expired")
	}
	return nil
}

func (capability ApprovalCapability) CanonicalBytes() ([]byte, error) {
	intentHash, err := capability.validate()
	if err != nil {
		return nil, err
	}
	stateWitness, err := decodeCanonicalDigest("state_witness", capability.StateWitness)
	if err != nil {
		return nil, err
	}
	var payload bytes.Buffer
	payload.WriteString(approvalCapabilityDomain)
	writeCanonicalString(&payload, capability.CapabilityVersion)
	writeCanonicalString(&payload, capability.Purpose)
	writeCanonicalString(&payload, capability.ApprovalID)
	writeCanonicalString(&payload, capability.TenantID)
	writeCanonicalInt64(&payload, capability.CompanyID)
	payload.Write(intentHash)
	payload.Write(stateWitness)
	writeCanonicalString(&payload, capability.CommandID)
	writeCanonicalInt64(&payload, capability.DelegationGrantID)
	writeCanonicalString(&payload, capability.DelegatorSubject)
	writeCanonicalString(&payload, capability.AgentSubject)
	writeCanonicalString(&payload, capability.CreatorSubject)
	writeCanonicalInt64(&payload, capability.ApproverUserID)
	writeCanonicalString(&payload, capability.ApproverSubject)
	writeCanonicalString(&payload, capability.RequiredPermission)
	writeCanonicalInt64(&payload, capability.IssuancePolicyRevision)
	writeCanonicalInt64(&payload, capability.IssuedAt)
	writeCanonicalInt64(&payload, capability.ExpiresAt)
	writeCanonicalString(&payload, capability.OneTimeID)
	return payload.Bytes(), nil
}

func (capability ApprovalCapability) validate() ([]byte, error) {
	if capability.CapabilityVersion != ApprovalCapabilityVersion || capability.Purpose != ApprovalCapabilityPurpose || capability.RequiredPermission != ApprovalRequiredPermission {
		return nil, errors.New("unsupported approval capability version, purpose or permission")
	}
	if err := validateOpaqueBase64URL("approval_id", capability.ApprovalID, 16); err != nil {
		return nil, err
	}
	if err := validateCanonicalText("tenant_id", capability.TenantID); err != nil {
		return nil, err
	}
	if capability.CompanyID <= 0 || capability.DelegationGrantID <= 0 || capability.ApproverUserID <= 0 || capability.IssuancePolicyRevision < 0 {
		return nil, errors.New("approval capability integer binding is invalid")
	}
	intentHash, err := decodeCanonicalDigest("intent_hash", capability.IntentHash)
	if err != nil {
		return nil, err
	}
	if err := validateCommandID(capability.CommandID); err != nil {
		return nil, err
	}
	if err := validateCanonicalSubject("delegator_subject", capability.DelegatorSubject, "user:"); err != nil {
		return nil, err
	}
	if err := validateCanonicalSubject("agent_subject", capability.AgentSubject, "agent:"); err != nil {
		return nil, err
	}
	if err := validateCanonicalSubject("creator_subject", capability.CreatorSubject, "user:"); err != nil {
		return nil, err
	}
	if err := validateCanonicalSubject("approver_subject", capability.ApproverSubject, "user:"); err != nil {
		return nil, err
	}
	if capability.ApproverSubject == capability.DelegatorSubject || capability.ApproverSubject == capability.CreatorSubject || capability.ApproverSubject == capability.AgentSubject {
		return nil, errors.New("approval capability violates separation of duties")
	}
	if capability.IssuedAt <= 0 || capability.ExpiresAt <= capability.IssuedAt || capability.ExpiresAt-capability.IssuedAt > int64(MaxApprovalCapabilityTTL/time.Second) {
		return nil, errors.New("approval capability validity window is invalid")
	}
	if err := validateOpaqueBase64URL("one_time_id", capability.OneTimeID, 32); err != nil {
		return nil, err
	}
	return intentHash, nil
}

func validateOpaqueBase64URL(field, value string, size int) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != size || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return fmt.Errorf("%s must be canonical unpadded base64url for %d bytes", field, size)
	}
	return nil
}

func randomBase64URL(source io.Reader, size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
