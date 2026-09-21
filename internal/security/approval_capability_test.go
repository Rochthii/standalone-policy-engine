package security

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func testApprovalCapability() ApprovalCapability {
	return ApprovalCapability{
		ApprovalID:             base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16)),
		TenantID:               "tenant-a",
		CompanyID:              17,
		IntentHash:             strings.Repeat("a", 64),
		StateWitness:           strings.Repeat("b", 64),
		CommandID:              base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)),
		DelegationGrantID:      42,
		DelegatorSubject:       "user:delegator",
		AgentSubject:           "agent:procurement",
		CreatorSubject:         "user:creator",
		ApproverUserID:         77,
		ApproverSubject:        "user:approver",
		IssuancePolicyRevision: 9,
	}
}

func testApprovalManager(t *testing.T) *ApprovalCapabilityManager {
	t.Helper()
	manager, err := NewApprovalCapabilityManagerWithKeyring(
		"approval-2026",
		map[string]string{
			"approval-2025": "previous-approval-secret-at-least-32-characters",
			"approval-2026": "current-approval-secret-at-least-32-characters",
		},
		15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return time.Unix(1_800_000_000, 0).UTC() }
	manager.random = bytes.NewReader(bytes.Repeat([]byte{3}, 32))
	return manager
}

func TestApprovalCapabilityIssueAndVerify(t *testing.T) {
	manager := testApprovalManager(t)
	capability, envelope, err := manager.Issue(testApprovalCapability(), manager.now().Unix()+3600)
	if err != nil {
		t.Fatal(err)
	}
	if capability.CapabilityVersion != ApprovalCapabilityVersion || capability.Purpose != ApprovalCapabilityPurpose || capability.RequiredPermission != ApprovalRequiredPermission {
		t.Fatalf("fixed capability fields missing: %+v", capability)
	}
	if capability.ExpiresAt != capability.IssuedAt+900 || envelope.Algorithm != ApprovalCapabilityAlgorithm || envelope.KeyID != "approval-2026" {
		t.Fatalf("unexpected issued envelope: capability=%+v envelope=%+v", capability, envelope)
	}
	if err := manager.Verify(capability, envelope); err != nil {
		t.Fatalf("verify issued capability: %v", err)
	}
}

func TestApprovalCapabilityRejectsTamperExpiryAndKeyConfusion(t *testing.T) {
	manager := testApprovalManager(t)
	capability, envelope, err := manager.Issue(testApprovalCapability(), manager.now().Unix()+3600)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		capability ApprovalCapability
		envelope   ApprovalCapabilityEnvelope
		advance    time.Duration
	}{
		{name: "intent", capability: func() ApprovalCapability {
			value := capability
			value.IntentHash = strings.Repeat("c", 64)
			return value
		}(), envelope: envelope},
		{name: "purpose", capability: func() ApprovalCapability { value := capability; value.Purpose = "delegation"; return value }(), envelope: envelope},
		{name: "algorithm", capability: capability, envelope: func() ApprovalCapabilityEnvelope { value := envelope; value.Algorithm = "none"; return value }()},
		{name: "unknown approval key", capability: capability, envelope: func() ApprovalCapabilityEnvelope { value := envelope; value.KeyID = "delegation-2026"; return value }()},
		{name: "signature", capability: capability, envelope: func() ApprovalCapabilityEnvelope {
			value := envelope
			value.Signature = append([]byte(nil), envelope.Signature...)
			value.Signature[0] ^= 0xff
			return value
		}()},
		{name: "expired", capability: capability, envelope: envelope, advance: 16 * time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalNow := manager.now
			manager.now = func() time.Time { return originalNow().Add(test.advance) }
			defer func() { manager.now = originalNow }()
			if err := manager.Verify(test.capability, test.envelope); err == nil {
				t.Fatal("expected capability verification to fail")
			}
		})
	}
}

func TestApprovalCapabilityRejectsSoDAndExpiredDelegation(t *testing.T) {
	manager := testApprovalManager(t)
	invalid := testApprovalCapability()
	invalid.ApproverSubject = invalid.DelegatorSubject
	if _, _, err := manager.Issue(invalid, manager.now().Unix()+3600); err == nil {
		t.Fatal("expected SoD collision to fail")
	}
	if _, _, err := manager.Issue(testApprovalCapability(), manager.now().Unix()); err == nil {
		t.Fatal("expected expired delegation to fail")
	}
}
