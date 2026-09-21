package security

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const (
	CanonicalBusinessIntentVersion = "cbi.v1"
	CanonicalIntentProofVersion    = "v2"
	PurchaseOrderResourceType      = "purchase.order"
	ConfirmPurchaseOrderAction     = "action:CONFIRM_PURCHASE_ORDER"

	canonicalIntentDomain = "PDP-CANONICAL-BUSINESS-INTENT"
	stateWitnessDomain    = "PDP-ODOO-PO-STATE-V1"
)

// CanonicalBusinessIntent is the exact persisted purchase-order command bound
// by a V2 delegation proof. Protected values must be reconstructed by the PEP.
type CanonicalBusinessIntent struct {
	IntentVersion      string `json:"intent_version"`
	TenantID           string `json:"tenant_id"`
	CompanyID          int64  `json:"company_id"`
	ResourceType       string `json:"resource_type"`
	ResourceID         int64  `json:"resource_id"`
	Action             string `json:"action"`
	VendorID           int64  `json:"vendor_id"`
	CurrencyCode       string `json:"currency_code"`
	CurrencyScale      int64  `json:"currency_scale"`
	AmountMinor        int64  `json:"amount_minor"`
	LineDigest         string `json:"line_digest"`
	RecordState        string `json:"record_state"`
	RecordWriteVersion string `json:"record_write_version"`
	StateWitness       string `json:"state_witness"`
	CreatorSubject     string `json:"creator_subject"`
	DelegationGrantID  int64  `json:"delegation_grant_id"`
	DelegatorSubject   string `json:"delegator_subject"`
	AgentSubject       string `json:"agent_subject"`
	CommandID          string `json:"command_id"`
	ProofVersion       string `json:"proof_version"`
}

// Validate rejects missing, ambiguous or non-canonical values and verifies
// that the supplied state witness matches the material fields.
func (intent CanonicalBusinessIntent) Validate() error {
	if err := intent.validateFields(); err != nil {
		return err
	}
	provided, err := decodeCanonicalDigest("state_witness", intent.StateWitness)
	if err != nil {
		return err
	}
	expected, err := intent.computeStateWitness()
	if err != nil {
		return err
	}
	if !bytes.Equal(provided, expected[:]) {
		return errors.New("state_witness does not match material state")
	}
	return nil
}

func (intent CanonicalBusinessIntent) validateFields() error {
	if intent.IntentVersion != CanonicalBusinessIntentVersion {
		return errors.New("unsupported canonical business intent version")
	}
	if err := validateCanonicalText("tenant_id", intent.TenantID); err != nil {
		return err
	}
	if intent.CompanyID <= 0 || intent.ResourceID <= 0 || intent.VendorID <= 0 || intent.DelegationGrantID <= 0 {
		return errors.New("company, resource, vendor and delegation grant IDs must be positive")
	}
	if intent.ResourceType != PurchaseOrderResourceType {
		return errors.New("unsupported canonical resource type")
	}
	if intent.Action != ConfirmPurchaseOrderAction {
		return errors.New("unsupported canonical action")
	}
	if len(intent.CurrencyCode) != 3 {
		return errors.New("currency_code must be three uppercase ASCII letters")
	}
	for _, value := range intent.CurrencyCode {
		if value < 'A' || value > 'Z' {
			return errors.New("currency_code must be three uppercase ASCII letters")
		}
	}
	if intent.CurrencyScale < 0 || intent.CurrencyScale > 6 {
		return errors.New("currency_scale must be between 0 and 6")
	}
	if intent.AmountMinor < 0 {
		return errors.New("amount_minor must be non-negative")
	}
	if _, err := decodeCanonicalDigest("line_digest", intent.LineDigest); err != nil {
		return err
	}
	if err := validateCanonicalText("record_state", intent.RecordState); err != nil {
		return err
	}
	if err := validateCanonicalTimestamp("record_write_version", intent.RecordWriteVersion); err != nil {
		return err
	}
	if err := validateCanonicalSubject("creator_subject", intent.CreatorSubject, "user:"); err != nil {
		return err
	}
	if err := validateCanonicalSubject("delegator_subject", intent.DelegatorSubject, "user:"); err != nil {
		return err
	}
	if err := validateCanonicalSubject("agent_subject", intent.AgentSubject, "agent:"); err != nil {
		return err
	}
	if err := validateCommandID(intent.CommandID); err != nil {
		return err
	}
	if intent.ProofVersion != CanonicalIntentProofVersion {
		return errors.New("unsupported canonical proof version")
	}
	return nil
}

// RefreshStateWitness computes the witness after all material fields are set.
func (intent *CanonicalBusinessIntent) RefreshStateWitness() error {
	if intent == nil {
		return errors.New("canonical business intent is nil")
	}
	if err := intent.validateFields(); err != nil {
		return err
	}
	witness, err := intent.computeStateWitness()
	if err != nil {
		return err
	}
	intent.StateWitness = hex.EncodeToString(witness[:])
	return nil
}

func (intent CanonicalBusinessIntent) computeStateWitness() ([sha256.Size]byte, error) {
	lineDigest, err := decodeCanonicalDigest("line_digest", intent.LineDigest)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	var payload bytes.Buffer
	payload.WriteString(stateWitnessDomain)
	writeCanonicalString(&payload, intent.TenantID)
	writeCanonicalInt64(&payload, intent.CompanyID)
	writeCanonicalString(&payload, intent.ResourceType)
	writeCanonicalInt64(&payload, intent.ResourceID)
	writeCanonicalInt64(&payload, intent.VendorID)
	writeCanonicalString(&payload, intent.CurrencyCode)
	writeCanonicalInt64(&payload, intent.CurrencyScale)
	writeCanonicalInt64(&payload, intent.AmountMinor)
	payload.Write(lineDigest)
	writeCanonicalString(&payload, intent.RecordState)
	writeCanonicalString(&payload, intent.RecordWriteVersion)
	writeCanonicalInt64(&payload, intent.DelegationGrantID)
	writeCanonicalString(&payload, intent.CreatorSubject)
	return sha256.Sum256(payload.Bytes()), nil
}

// CanonicalBytes encodes every field in normative ordinal order.
func (intent CanonicalBusinessIntent) CanonicalBytes() ([]byte, error) {
	if err := intent.Validate(); err != nil {
		return nil, err
	}
	lineDigest, _ := decodeCanonicalDigest("line_digest", intent.LineDigest)
	stateWitness, _ := decodeCanonicalDigest("state_witness", intent.StateWitness)
	var payload bytes.Buffer
	payload.WriteString(canonicalIntentDomain)
	stringsInOrder := []string{intent.IntentVersion, intent.TenantID}
	for _, value := range stringsInOrder {
		writeCanonicalString(&payload, value)
	}
	writeCanonicalInt64(&payload, intent.CompanyID)
	writeCanonicalString(&payload, intent.ResourceType)
	writeCanonicalInt64(&payload, intent.ResourceID)
	writeCanonicalString(&payload, intent.Action)
	writeCanonicalInt64(&payload, intent.VendorID)
	writeCanonicalString(&payload, intent.CurrencyCode)
	writeCanonicalInt64(&payload, intent.CurrencyScale)
	writeCanonicalInt64(&payload, intent.AmountMinor)
	payload.Write(lineDigest)
	writeCanonicalString(&payload, intent.RecordState)
	writeCanonicalString(&payload, intent.RecordWriteVersion)
	payload.Write(stateWitness)
	writeCanonicalString(&payload, intent.CreatorSubject)
	writeCanonicalInt64(&payload, intent.DelegationGrantID)
	writeCanonicalString(&payload, intent.DelegatorSubject)
	writeCanonicalString(&payload, intent.AgentSubject)
	writeCanonicalString(&payload, intent.CommandID)
	writeCanonicalString(&payload, intent.ProofVersion)
	return payload.Bytes(), nil
}

// IntentHash returns SHA-256 over the validated canonical bytes.
func (intent CanonicalBusinessIntent) IntentHash() ([sha256.Size]byte, error) {
	payload, err := intent.CanonicalBytes()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(payload), nil
}
