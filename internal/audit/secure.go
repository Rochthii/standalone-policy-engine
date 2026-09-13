package audit

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"standalone-policy-engine/internal/security"
)

const auditPayloadVersion = 1

type AuditPayload struct {
	Subject  string            `json:"subject"`
	Action   string            `json:"action"`
	Resource string            `json:"resource"`
	Context  map[string]string `json:"context,omitempty"`
}

type auditAAD struct {
	Version         int    `json:"v"`
	AuditID         string `json:"audit_id"`
	Timestamp       int64  `json:"ts"`
	RevisionID      uint64 `json:"rev"`
	TenantID        string `json:"tenant_id"`
	Decision        string `json:"decision"`
	MatchedPolicyID string `json:"matched_policy_id,omitempty"`
	RequestID       string `json:"request_id,omitempty"`
	TraceID         string `json:"trace_id,omitempty"`
	KeyID           string `json:"key_id"`
}

func sealAuditEntry(crypto *security.EnvelopeCrypto, entry *LogEntry) error {
	if crypto == nil {
		return errors.New("audit envelope crypto is required")
	}
	auditID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	entry.AuditID = auditID.String()
	entry.PayloadVersion = auditPayloadVersion
	entry.KeyID = crypto.ActiveKeyID()
	entry.RequestID = entry.Context["request_id"]
	entry.TraceID = entry.Context["trace_id"]

	payload, err := json.Marshal(AuditPayload{
		Subject: entry.Subject, Action: entry.Action, Resource: entry.Resource, Context: entry.Context,
	})
	if err != nil {
		return err
	}
	aad, err := auditEntryAAD(entry)
	if err != nil {
		return err
	}
	ciphertext, encryptedDEK, keyID, err := crypto.EncryptWithAAD(payload, aad)
	if err != nil {
		return err
	}
	entry.KeyID = keyID
	entry.EncryptedPayload = ciphertext
	entry.EncryptedDEK = encryptedDEK
	entry.IntegrityTag, err = crypto.IntegrityTag(keyID, aad, []byte(encryptedDEK), []byte(ciphertext))
	if err != nil {
		return err
	}
	entry.IsEncrypted = true
	entry.Subject, entry.Action, entry.Resource, entry.Context = "", "", "", nil
	return nil
}

func VerifyAndDecryptEntry(crypto *security.EnvelopeCrypto, entry *LogEntry) (*AuditPayload, error) {
	if entry == nil || !entry.IsEncrypted || entry.PayloadVersion != auditPayloadVersion {
		return nil, errors.New("unsupported or unencrypted audit entry")
	}
	aad, err := auditEntryAAD(entry)
	if err != nil {
		return nil, err
	}
	if err := crypto.VerifyIntegrityTag(entry.IntegrityTag, entry.KeyID, aad, []byte(entry.EncryptedDEK), []byte(entry.EncryptedPayload)); err != nil {
		return nil, err
	}
	plaintext, err := crypto.DecryptWithAAD(entry.EncryptedPayload, entry.EncryptedDEK, entry.KeyID, aad)
	if err != nil {
		return nil, err
	}
	var payload AuditPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func auditEntryAAD(entry *LogEntry) ([]byte, error) {
	return json.Marshal(auditAAD{
		Version: entry.PayloadVersion, AuditID: entry.AuditID, Timestamp: entry.Timestamp,
		RevisionID: entry.RevisionID, TenantID: entry.TenantID, Decision: entry.Decision,
		MatchedPolicyID: entry.MatchedPolicyID, RequestID: entry.RequestID, TraceID: entry.TraceID,
		KeyID: entry.KeyID,
	})
}
