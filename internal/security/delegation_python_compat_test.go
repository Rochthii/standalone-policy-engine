package security

import "testing"

func TestDelegationProofPythonCompatibilityVector(t *testing.T) {
	manager, err := NewDelegationManagerWithKeyring("key-2026-09", map[string]string{
		"key-2026-09": "test-delegation-secret-at-least-32-characters",
	})
	if err != nil {
		t.Fatalf("create key ring: %v", err)
	}

	proof, err := manager.GenerateProof(DelegationProofInput{
		TenantID:        "tenant-a",
		GrantID:         "grant-42",
		Delegator:       "user:bob",
		Agent:           "agent:procurement_copilot",
		Action:          "action:APPROVE_PURCHASE_ORDER",
		Resource:        "purchase_order:17",
		Amount:          "2001",
		DelegationChain: "user:bob,agent:procurement_copilot",
		CreatorID:       "user:alice",
		ToolContext:     "tool:auto_confirm_po",
		ExecutionMode:   "autonomous_run",
		Nonce:           "nonce-0001",
		IssuedAt:        1800000000,
		ValidUntil:      1800003600,
	})
	if err != nil {
		t.Fatalf("generate proof: %v", err)
	}
	const expected = "v1.key-2026-09.c16450b09b9988d55e77d57915b055358e21e057e45ffe2d5474ac940a92faf3"
	if proof != expected {
		t.Fatalf("proof does not match Python compatibility vector: got %s", proof)
	}
}
