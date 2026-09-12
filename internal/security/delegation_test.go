package security

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func validDelegationInput() DelegationProofInput {
	now := time.Now().Unix()
	return DelegationProofInput{
		TenantID:        "tenant-a",
		GrantID:         "grant-42",
		Delegator:       "user:bob",
		Agent:           "agent:procurement_copilot",
		Action:          "action:APPROVE_PURCHASE_ORDER",
		Resource:        "purchase_order:PO-42",
		Amount:          "1500",
		DelegationChain: "user:bob,agent:procurement_copilot",
		CreatorID:       "user:alice",
		ToolContext:     "tool:auto_confirm_po",
		ExecutionMode:   "autonomous_run",
		Nonce:           "nonce-42",
		IssuedAt:        now,
		ValidUntil:      now + int64(time.Hour/time.Second),
	}
}

func TestDelegationManager_VerifyProof_Success(t *testing.T) {
	mgr := NewDelegationManager()
	input := validDelegationInput()
	proof, err := mgr.GenerateProof(input)
	if err != nil {
		t.Fatalf("GenerateProof failed: %v", err)
	}
	if err := mgr.VerifyProof(input, proof); err != nil {
		t.Fatalf("expected proof verification to succeed: %v", err)
	}
}

func TestDelegationManager_KeyRotationOverlapAndRetirement(t *testing.T) {
	oldKeys := map[string]string{
		"key-2026-09": "old-delegation-secret-at-least-32-characters",
	}
	oldManager, err := NewDelegationManagerWithKeyring("key-2026-09", oldKeys)
	if err != nil {
		t.Fatalf("create old key ring: %v", err)
	}
	input := validDelegationInput()
	oldProof, err := oldManager.GenerateProof(input)
	if err != nil {
		t.Fatalf("generate old proof: %v", err)
	}
	if !strings.HasPrefix(oldProof, DelegationProofVersion+".key-2026-09.") {
		t.Fatalf("proof must carry the signing key ID, got %q", oldProof)
	}

	rotatedManager, err := NewDelegationManagerWithKeyring("key-2026-10", map[string]string{
		"key-2026-09": "old-delegation-secret-at-least-32-characters",
		"key-2026-10": "new-delegation-secret-at-least-32-characters",
	})
	if err != nil {
		t.Fatalf("create overlapping key ring: %v", err)
	}
	if err := rotatedManager.VerifyProof(input, oldProof); err != nil {
		t.Fatalf("overlap window must accept a proof signed by the retained old key: %v", err)
	}
	newProof, err := rotatedManager.GenerateProof(input)
	if err != nil {
		t.Fatalf("generate proof with active rotated key: %v", err)
	}
	if !strings.HasPrefix(newProof, DelegationProofVersion+".key-2026-10.") {
		t.Fatalf("new proof must use the active rotated key, got %q", newProof)
	}

	retiredManager, err := NewDelegationManagerWithKeyring("key-2026-10", map[string]string{
		"key-2026-10": "new-delegation-secret-at-least-32-characters",
	})
	if err != nil {
		t.Fatalf("create retired key ring: %v", err)
	}
	if err := retiredManager.VerifyProof(input, oldProof); err == nil {
		t.Fatal("proof signed by a retired key must fail closed")
	}
	if err := retiredManager.VerifyProof(input, newProof); err != nil {
		t.Fatalf("proof signed by the current key must remain valid: %v", err)
	}
}

func TestDelegationManager_RejectsInvalidKeyringAndUnknownKey(t *testing.T) {
	if _, err := NewDelegationManagerWithKeyring("missing", map[string]string{"other": "secret"}); err == nil {
		t.Fatal("active key must exist in the key ring")
	}
	if _, err := NewDelegationManagerWithKeyring("bad.key", map[string]string{"bad.key": "secret"}); err == nil {
		t.Fatal("key IDs containing proof separators must be rejected")
	}

	manager, err := NewDelegationManagerWithKeyring("key-a", map[string]string{
		"key-a": "delegation-secret-at-least-32-characters",
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	input := validDelegationInput()
	proof, err := manager.GenerateProof(input)
	if err != nil {
		t.Fatalf("generate proof: %v", err)
	}
	unknownKeyProof := strings.Replace(proof, ".key-a.", ".key-b.", 1)
	if err := manager.VerifyProof(input, unknownKeyProof); err == nil {
		t.Fatal("unknown key ID must fail closed")
	}

	sharedSecret := "shared-delegation-secret-at-least-32-characters"
	confusionManager, err := NewDelegationManagerWithKeyring("key-a", map[string]string{
		"key-a": sharedSecret,
		"key-b": sharedSecret,
	})
	if err != nil {
		t.Fatalf("create key-confusion manager: %v", err)
	}
	proof, err = confusionManager.GenerateProof(input)
	if err != nil {
		t.Fatalf("generate key-confusion proof: %v", err)
	}
	changedKeyID := strings.Replace(proof, ".key-a.", ".key-b.", 1)
	if err := confusionManager.VerifyProof(input, changedKeyID); err == nil {
		t.Fatal("changing key ID must fail even when two IDs temporarily share secret bytes")
	}
}

func TestDelegationManager_VerifyProof_BindsCompleteTuple(t *testing.T) {
	mgr := NewDelegationManager()
	input := validDelegationInput()
	proof, err := mgr.GenerateProof(input)
	if err != nil {
		t.Fatalf("GenerateProof failed: %v", err)
	}

	tamperCases := map[string]func(*DelegationProofInput){
		"tenant":    func(v *DelegationProofInput) { v.TenantID = "tenant-b" },
		"grant":     func(v *DelegationProofInput) { v.GrantID = "grant-99" },
		"delegator": func(v *DelegationProofInput) { v.Delegator = "user:eve"; v.DelegationChain = "user:eve," + v.Agent },
		"agent": func(v *DelegationProofInput) {
			v.Agent = "agent:other"
			v.DelegationChain = v.Delegator + ",agent:other"
		},
		"action":      func(v *DelegationProofInput) { v.Action = "action:DELETE" },
		"resource":    func(v *DelegationProofInput) { v.Resource = "purchase_order:PO-99" },
		"amount":      func(v *DelegationProofInput) { v.Amount = "50000" },
		"creator":     func(v *DelegationProofInput) { v.CreatorID = "user:bob" },
		"tool":        func(v *DelegationProofInput) { v.ToolContext = "tool:other" },
		"mode":        func(v *DelegationProofInput) { v.ExecutionMode = "interactive" },
		"nonce":       func(v *DelegationProofInput) { v.Nonce = "nonce-replayed" },
		"issued_at":   func(v *DelegationProofInput) { v.IssuedAt-- },
		"valid_until": func(v *DelegationProofInput) { v.ValidUntil-- },
	}

	for name, tamper := range tamperCases {
		t.Run(name, func(t *testing.T) {
			changed := input
			tamper(&changed)
			if err := mgr.VerifyProof(changed, proof); err == nil {
				t.Fatal("tampered delegation tuple was accepted")
			}
		})
	}
}

func TestDelegationManager_VerifyProof_ExpiredTTL(t *testing.T) {
	mgr := NewDelegationManager()
	input := validDelegationInput()
	input.IssuedAt = time.Now().Add(-time.Hour).Unix()
	input.ValidUntil = time.Now().Add(-time.Minute).Unix()
	proof, err := mgr.GenerateProof(input)
	if err != nil {
		t.Fatalf("GenerateProof failed: %v", err)
	}
	if err := mgr.VerifyProof(input, proof); err == nil {
		t.Fatal("expected expired proof to be rejected")
	}
}

func TestDelegationManager_RejectsOverlongTTLAndDelimiterConfusion(t *testing.T) {
	mgr := NewDelegationManager()
	overlong := validDelegationInput()
	overlong.ValidUntil = overlong.IssuedAt + int64((MaxDelegationTTL+time.Second)/time.Second)
	if _, err := mgr.GenerateProof(overlong); err == nil {
		t.Fatal("expected overlong delegation TTL to be rejected")
	}

	left := validDelegationInput()
	left.Amount = "1|2"
	right := left
	right.Amount = "1"
	right.CreatorID = "2|" + right.CreatorID
	if string(left.CanonicalBytes()) == string(right.CanonicalBytes()) {
		t.Fatal("length-prefixed canonical encoding must distinguish delimiter placement")
	}
}

func TestDelegationManager_RevocationMap_O1(t *testing.T) {
	mgr := NewDelegationManager()
	tenantID := "tenant-a"
	grantID := "grant-uuid-101"

	if mgr.IsRevoked(tenantID, grantID) {
		t.Fatalf("grantID should not be revoked initially")
	}

	revokedAt := mgr.Revoke(tenantID, grantID)
	if revokedAt <= 0 {
		t.Fatalf("revokedAt timestamp should be greater than 0")
	}

	if !mgr.IsRevoked(tenantID, grantID) {
		t.Fatalf("grantID should be revoked after calling Revoke()")
	}
}

func TestDelegationManager_RevocationIsTenantScoped(t *testing.T) {
	mgr := NewDelegationManager()
	grantID := "shared-grant-id"

	mgr.Revoke("tenant-a", grantID)
	if !mgr.IsRevoked("tenant-a", grantID) {
		t.Fatal("revocation must apply in the owning tenant")
	}
	if mgr.IsRevoked("tenant-b", grantID) {
		t.Fatal("same grant ID in another tenant must not be revoked")
	}
}

func TestDelegationManager_RevocationCleanupIsBoundedByTTL(t *testing.T) {
	mgr := newDelegationManagerWithRevocationTTL("test-secret", time.Hour)
	expiredKey := revocationKey{tenantID: "tenant-a", grantID: "expired"}
	mgr.revocationMap.Store(expiredKey, revocationRecord{
		revokedAt: time.Now().Add(-2 * time.Hour).Unix(),
		expiresAt: time.Now().Add(-time.Hour).UnixNano(),
	})

	mgr.Revoke("tenant-a", "current")
	if _, exists := mgr.revocationMap.Load(expiredKey); exists {
		t.Fatal("expired revocation must be removed on the next write")
	}
	if !mgr.IsRevoked("tenant-a", "current") {
		t.Fatal("current revocation must remain active")
	}
}

func TestDelegationManager_ConcurrentRace(t *testing.T) {
	mgr := NewDelegationManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			grantID := strconv.Itoa(id)
			mgr.IsRevoked("tenant-a", grantID)
			mgr.Revoke("tenant-a", grantID)
			mgr.IsRevoked("tenant-a", grantID)
		}(i)
	}

	wg.Wait()
}
