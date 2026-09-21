package security

import (
	"strings"
	"testing"
	"time"
)

func validDelegationV2Input(t *testing.T) DelegationProofV2Input {
	t.Helper()
	now := time.Now().Unix()
	return DelegationProofV2Input{
		Intent:     validCanonicalBusinessIntent(t),
		IssuedAt:   now,
		ValidUntil: now + int64(time.Hour/time.Second),
	}
}

func TestDelegationManagerV2AcceptsValidIntent(t *testing.T) {
	manager, err := NewDelegationManagerWithKeyring("key-2026-09", map[string]string{
		"key-2026-09": "test-delegation-secret-at-least-32-characters",
	})
	if err != nil {
		t.Fatalf("create key ring: %v", err)
	}
	input := validDelegationV2Input(t)
	proof, err := manager.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("generate V2 proof: %v", err)
	}
	if !strings.HasPrefix(proof, DelegationProofV2Version+".key-2026-09.") {
		t.Fatalf("V2 proof does not carry version and key ID: %q", proof)
	}
	if err := manager.VerifyProofV2(input, proof); err != nil {
		t.Fatalf("verify valid V2 proof: %v", err)
	}
}

func TestDelegationManagerV2BindsIntentAndMetadata(t *testing.T) {
	manager := NewDelegationManagerWithSecret("test-delegation-secret-at-least-32-characters")
	input := validDelegationV2Input(t)
	proof, err := manager.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("generate V2 proof: %v", err)
	}

	cases := map[string]func(*DelegationProofV2Input){
		"amount": func(v *DelegationProofV2Input) {
			v.Intent.AmountMinor++
			if err := v.Intent.RefreshStateWitness(); err != nil {
				t.Fatalf("refresh amount witness: %v", err)
			}
		},
		"grant": func(v *DelegationProofV2Input) {
			v.Intent.DelegationGrantID++
			if err := v.Intent.RefreshStateWitness(); err != nil {
				t.Fatalf("refresh grant witness: %v", err)
			}
		},
		"agent":       func(v *DelegationProofV2Input) { v.Intent.AgentSubject = "agent:other" },
		"issued at":   func(v *DelegationProofV2Input) { v.IssuedAt-- },
		"valid until": func(v *DelegationProofV2Input) { v.ValidUntil-- },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			if err := manager.VerifyProofV2(changed, proof); err == nil {
				t.Fatal("changed V2 proof input was accepted")
			}
		})
	}
}

func TestDelegationManagerV2RejectsDowngradeAndKeyConfusion(t *testing.T) {
	manager, err := NewDelegationManagerWithKeyring("key-a", map[string]string{
		"key-a": "same-test-secret-at-least-32-characters",
		"key-b": "same-test-secret-at-least-32-characters",
	})
	if err != nil {
		t.Fatalf("create key ring: %v", err)
	}
	input := validDelegationV2Input(t)
	v2Proof, err := manager.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("generate V2 proof: %v", err)
	}
	if err := manager.VerifyProofV2(input, strings.Replace(v2Proof, ".key-a.", ".key-b.", 1)); err == nil {
		t.Fatal("changed key ID was accepted even though canonical bytes bind it")
	}

	v1Input := validDelegationInput()
	v1Proof, err := manager.GenerateProof(v1Input)
	if err != nil {
		t.Fatalf("generate V1 proof: %v", err)
	}
	if err := manager.VerifyProofV2(input, v1Proof); err == nil {
		t.Fatal("V1 proof was accepted by V2 verifier")
	}
	if err := manager.VerifyProof(v1Input, v2Proof); err == nil {
		t.Fatal("V2 proof was accepted by V1 verifier")
	}
}

func TestDelegationManagerV2RejectsInvalidValidityAndSignature(t *testing.T) {
	manager := NewDelegationManagerWithSecret("test-delegation-secret-at-least-32-characters")
	input := validDelegationV2Input(t)
	overlong := input
	overlong.ValidUntil = overlong.IssuedAt + int64((MaxDelegationTTL+time.Second)/time.Second)
	if _, err := manager.GenerateProofV2(overlong); err == nil {
		t.Fatal("overlong V2 proof TTL was accepted")
	}

	proof, err := manager.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("generate V2 proof: %v", err)
	}
	parts := strings.Split(proof, ".")
	upperSignature := parts[0] + "." + parts[1] + "." + strings.ToUpper(parts[2])
	if err := manager.VerifyProofV2(input, upperSignature); err == nil {
		t.Fatal("non-canonical uppercase signature was accepted")
	}
}
