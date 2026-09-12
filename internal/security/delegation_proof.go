package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DelegationProofVersion = "v1"
	MaxDelegationTTL       = 24 * time.Hour
	delegationClockSkew    = 30 * time.Second
)

// DelegationProofInput is the complete authorization tuple protected by HMAC.
// Empty optional constraint fields are still encoded and therefore cannot be
// added or changed after the proof is issued.
type DelegationProofInput struct {
	TenantID        string
	GrantID         string
	Delegator       string
	Agent           string
	Action          string
	Resource        string
	Amount          string
	DelegationChain string
	CreatorID       string
	ToolContext     string
	ExecutionMode   string
	Nonce           string
	IssuedAt        int64
	ValidUntil      int64
}

func (input DelegationProofInput) validateStructure() error {
	required := map[string]string{
		"tenant_id":        input.TenantID,
		"grant_id":         input.GrantID,
		"delegator":        input.Delegator,
		"agent":            input.Agent,
		"action":           input.Action,
		"resource":         input.Resource,
		"delegation_chain": input.DelegationChain,
		"nonce":            input.Nonce,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing delegation field %s", field)
		}
	}
	if input.IssuedAt <= 0 || input.ValidUntil <= 0 || input.ValidUntil <= input.IssuedAt {
		return errors.New("invalid delegation validity window")
	}
	if time.Duration(input.ValidUntil-input.IssuedAt)*time.Second > MaxDelegationTTL {
		return fmt.Errorf("delegation TTL exceeds %s", MaxDelegationTTL)
	}
	chain := strings.Split(input.DelegationChain, ",")
	if len(chain) != 2 || chain[0] != input.Delegator || chain[1] != input.Agent {
		return errors.New("delegation chain must bind delegator directly to agent")
	}
	return nil
}

func (input DelegationProofInput) validateAt(now time.Time) error {
	if err := input.validateStructure(); err != nil {
		return err
	}
	nowUnix := now.Unix()
	if input.IssuedAt > now.Add(delegationClockSkew).Unix() {
		return errors.New("delegation proof is not active yet")
	}
	if nowUnix > input.ValidUntil {
		return errors.New("delegation proof has expired")
	}
	return nil
}

// CanonicalBytes uses fixed field order and uint32 length prefixes, avoiding
// delimiter-confusion collisions present in the historical pipe-joined string.
// Proof signing additionally binds the selected key ID through canonicalBytes.
func (input DelegationProofInput) CanonicalBytes() []byte {
	return input.canonicalBytes("")
}

func (input DelegationProofInput) canonicalBytes(keyID string) []byte {
	var payload bytes.Buffer
	payload.WriteString("PDP-DELEGATION-PROOF")
	writeCanonicalField(&payload, DelegationProofVersion)
	writeCanonicalField(&payload, keyID)
	writeCanonicalField(&payload, input.TenantID)
	writeCanonicalField(&payload, input.GrantID)
	writeCanonicalField(&payload, input.Delegator)
	writeCanonicalField(&payload, input.Agent)
	writeCanonicalField(&payload, input.Action)
	writeCanonicalField(&payload, input.Resource)
	writeCanonicalField(&payload, input.Amount)
	writeCanonicalField(&payload, input.DelegationChain)
	writeCanonicalField(&payload, input.CreatorID)
	writeCanonicalField(&payload, input.ToolContext)
	writeCanonicalField(&payload, input.ExecutionMode)
	writeCanonicalField(&payload, input.Nonce)
	_ = binary.Write(&payload, binary.BigEndian, input.IssuedAt)
	_ = binary.Write(&payload, binary.BigEndian, input.ValidUntil)
	return payload.Bytes()
}

func writeCanonicalField(payload *bytes.Buffer, value string) {
	_ = binary.Write(payload, binary.BigEndian, uint32(len(value)))
	payload.WriteString(value)
}

// GenerateProof signs a structurally valid tuple. Expiration is checked by
// VerifyProof, which also allows deterministic tests using historical tuples.
func (m *DelegationManager) GenerateProof(input DelegationProofInput) (string, error) {
	if m == nil || !validDelegationKeyID(m.activeKeyID) {
		return "", errors.New("delegation signer is not configured")
	}
	secret := m.signingKeys[m.activeKeyID]
	if len(secret) == 0 {
		return "", errors.New("active delegation signing key is not configured")
	}
	if err := input.validateStructure(); err != nil {
		return "", err
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(input.canonicalBytes(m.activeKeyID))
	return DelegationProofVersion + "." + m.activeKeyID + "." + hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyProof validates the full tuple, validity window and constant-time HMAC.
func (m *DelegationManager) VerifyProof(input DelegationProofInput, proof string) error {
	if m == nil || len(m.signingKeys) == 0 {
		return errors.New("delegation verifier is not configured")
	}
	if err := input.validateAt(time.Now()); err != nil {
		return err
	}
	parts := strings.Split(proof, ".")
	if len(parts) != 3 || parts[0] != DelegationProofVersion || !validDelegationKeyID(parts[1]) {
		return errors.New("unsupported delegation proof version")
	}
	secret, exists := m.signingKeys[parts[1]]
	if !exists || len(secret) == 0 {
		return errors.New("unknown delegation signing key")
	}
	provided, err := hex.DecodeString(parts[2])
	if err != nil {
		return errors.New("malformed delegation signature")
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(input.canonicalBytes(parts[1]))
	if !hmac.Equal(h.Sum(nil), provided) {
		return errors.New("delegation signature mismatch")
	}
	return nil
}
