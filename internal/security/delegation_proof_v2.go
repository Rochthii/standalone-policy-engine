package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DelegationProofV2Version = "v2"
	delegationProofV2Domain  = "PDP-DELEGATION-PROOF-V2"
)

// DelegationProofV2Input keeps the complete CBI in the verifier API so callers
// cannot accidentally validate a stale, separately supplied intent hash.
type DelegationProofV2Input struct {
	Intent     CanonicalBusinessIntent
	IssuedAt   int64
	ValidUntil int64
}

func (input DelegationProofV2Input) validateStructure() error {
	if err := input.Intent.Validate(); err != nil {
		return fmt.Errorf("invalid canonical business intent: %w", err)
	}
	if input.IssuedAt <= 0 || input.ValidUntil <= input.IssuedAt {
		return errors.New("invalid V2 delegation validity window")
	}
	if input.ValidUntil-input.IssuedAt > int64(MaxDelegationTTL/time.Second) {
		return fmt.Errorf("V2 delegation TTL exceeds %s", MaxDelegationTTL)
	}
	return nil
}

func (input DelegationProofV2Input) validateAt(now time.Time) error {
	if err := input.validateStructure(); err != nil {
		return err
	}
	if input.IssuedAt > now.Add(delegationClockSkew).Unix() {
		return errors.New("V2 delegation proof is not active yet")
	}
	if now.Unix() > input.ValidUntil {
		return errors.New("V2 delegation proof has expired")
	}
	return nil
}

func (input DelegationProofV2Input) canonicalBytes(keyID string) ([]byte, error) {
	if err := input.validateStructure(); err != nil {
		return nil, err
	}
	intentHash, err := input.Intent.IntentHash()
	if err != nil {
		return nil, err
	}
	var payload bytes.Buffer
	payload.WriteString(delegationProofV2Domain)
	writeCanonicalString(&payload, DelegationProofV2Version)
	writeCanonicalString(&payload, keyID)
	payload.Write(intentHash[:])
	writeCanonicalInt64(&payload, input.Intent.DelegationGrantID)
	writeCanonicalString(&payload, input.Intent.AgentSubject)
	writeCanonicalInt64(&payload, input.IssuedAt)
	writeCanonicalInt64(&payload, input.ValidUntil)
	return payload.Bytes(), nil
}

// GenerateProofV2 signs a valid CBI while leaving the historical V1 API intact.
func (m *DelegationManager) GenerateProofV2(input DelegationProofV2Input) (string, error) {
	if m == nil || !validDelegationKeyID(m.activeKeyID) {
		return "", errors.New("V2 delegation signer is not configured")
	}
	secret := m.signingKeys[m.activeKeyID]
	if len(secret) == 0 {
		return "", errors.New("active V2 delegation signing key is not configured")
	}
	payload, err := input.canonicalBytes(m.activeKeyID)
	if err != nil {
		return "", err
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(payload)
	return DelegationProofV2Version + "." + m.activeKeyID + "." + hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyProofV2 validates V2 structure, time, key metadata and constant-time HMAC.
func (m *DelegationManager) VerifyProofV2(input DelegationProofV2Input, proof string) error {
	if m == nil || len(m.signingKeys) == 0 {
		return errors.New("V2 delegation verifier is not configured")
	}
	if err := input.validateAt(time.Now()); err != nil {
		return err
	}
	parts := strings.Split(proof, ".")
	if len(parts) != 3 || parts[0] != DelegationProofV2Version || !validDelegationKeyID(parts[1]) {
		return errors.New("unsupported V2 delegation proof version")
	}
	secret, exists := m.signingKeys[parts[1]]
	if !exists || len(secret) == 0 {
		return errors.New("unknown V2 delegation signing key")
	}
	if len(parts[2]) != sha256.Size*2 || parts[2] != strings.ToLower(parts[2]) {
		return errors.New("malformed V2 delegation signature")
	}
	provided, err := hex.DecodeString(parts[2])
	if err != nil {
		return errors.New("malformed V2 delegation signature")
	}
	payload, err := input.canonicalBytes(parts[1])
	if err != nil {
		return err
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(payload)
	if !hmac.Equal(h.Sum(nil), provided) {
		return errors.New("V2 delegation signature mismatch")
	}
	return nil
}
