package security

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func validCanonicalBusinessIntent(t *testing.T) CanonicalBusinessIntent {
	t.Helper()
	lineDigest := sha256.Sum256([]byte("canonical purchase order lines"))
	commandID := sha256.Sum256([]byte("command-42"))
	intent := CanonicalBusinessIntent{
		IntentVersion:      CanonicalBusinessIntentVersion,
		TenantID:           "tenant-a",
		CompanyID:          7,
		ResourceType:       PurchaseOrderResourceType,
		ResourceID:         42,
		Action:             ConfirmPurchaseOrderAction,
		VendorID:           19,
		CurrencyCode:       "USD",
		CurrencyScale:      2,
		AmountMinor:        123456,
		LineDigest:         hex.EncodeToString(lineDigest[:]),
		RecordState:        "to approve",
		RecordWriteVersion: "2026-09-20T10:11:12.123456Z",
		CreatorSubject:     "user:alice",
		DelegationGrantID:  81,
		DelegatorSubject:   "user:bob",
		AgentSubject:       "agent:procurement_copilot",
		CommandID:          base64.RawURLEncoding.EncodeToString(commandID[:]),
		ProofVersion:       CanonicalIntentProofVersion,
	}
	if err := intent.RefreshStateWitness(); err != nil {
		t.Fatalf("refresh valid state witness: %v", err)
	}
	return intent
}

func TestCanonicalBusinessIntentValidAndDeterministic(t *testing.T) {
	intent := validCanonicalBusinessIntent(t)
	first, err := intent.CanonicalBytes()
	if err != nil {
		t.Fatalf("canonicalize valid intent: %v", err)
	}
	second, err := intent.CanonicalBytes()
	if err != nil {
		t.Fatalf("canonicalize valid intent again: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("canonical encoding is not deterministic")
	}
	firstHash, err := intent.IntentHash()
	if err != nil {
		t.Fatalf("hash valid intent: %v", err)
	}
	secondHash := sha256.Sum256(first)
	if firstHash != secondHash {
		t.Fatal("intent hash is not SHA-256 of canonical bytes")
	}
	const expectedHash = "e9b71c9a22c05e85cbbe0e6c3a767acb99631fd08d3306ed9f01c6ea577ff28c"
	const expectedBytes = "UERQLUNBTk9OSUNBTC1CVVNJTkVTUy1JTlRFTlQAAAAGY2JpLnYxAAAACHRlbmFudC1hAAAAAAAAAAcAAAAOcHVyY2hhc2Uub3JkZXIAAAAAAAAAKgAAAB1hY3Rpb246Q09ORklSTV9QVVJDSEFTRV9PUkRFUgAAAAAAAAATAAAAA1VTRAAAAAAAAAACAAAAAAAB4kC89jzjrwo0cuYLHfRWVC9Gj1XoqkK1JP+TkHhxY1WQEgAAAAp0byBhcHByb3ZlAAAAGzIwMjYtMDktMjBUMTA6MTE6MTIuMTIzNDU2WolGC+uE0RTElP3xf75yvzJyzWGq0pa0x8dnbLezC9SqAAAACnVzZXI6YWxpY2UAAAAAAAAAUQAAAAh1c2VyOmJvYgAAABlhZ2VudDpwcm9jdXJlbWVudF9jb3BpbG90AAAAK01WV3pqYnlkQnhRVXZweC1RRlQ1ZEhJU0swdDk1Q3pudlRHUEVFcW5SRGsAAAACdjI="
	if hex.EncodeToString(firstHash[:]) != expectedHash {
		t.Fatalf("canonical intent hash changed: got %x", firstHash)
	}
	if base64.StdEncoding.EncodeToString(first) != expectedBytes {
		t.Fatal("canonical intent bytes changed")
	}
}

func TestCanonicalBusinessIntentRejectsMalformedFields(t *testing.T) {
	valid := validCanonicalBusinessIntent(t)
	cases := map[string]func(*CanonicalBusinessIntent){
		"missing tenant":       func(v *CanonicalBusinessIntent) { v.TenantID = "" },
		"trimmed tenant":       func(v *CanonicalBusinessIntent) { v.TenantID = " tenant-a" },
		"company range":        func(v *CanonicalBusinessIntent) { v.CompanyID = 0 },
		"wrong resource":       func(v *CanonicalBusinessIntent) { v.ResourceType = "sale.order" },
		"wrong action":         func(v *CanonicalBusinessIntent) { v.Action = "action:CANCEL" },
		"lowercase currency":   func(v *CanonicalBusinessIntent) { v.CurrencyCode = "usd" },
		"currency length":      func(v *CanonicalBusinessIntent) { v.CurrencyCode = "US" },
		"currency scale range": func(v *CanonicalBusinessIntent) { v.CurrencyScale = 7 },
		"negative amount":      func(v *CanonicalBusinessIntent) { v.AmountMinor = -1 },
		"uppercase line digest": func(v *CanonicalBusinessIntent) {
			v.LineDigest = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		},
		"short state witness":     func(v *CanonicalBusinessIntent) { v.StateWitness = "00" },
		"timestamp precision":     func(v *CanonicalBusinessIntent) { v.RecordWriteVersion = "2026-09-20T10:11:12Z" },
		"creator subject":         func(v *CanonicalBusinessIntent) { v.CreatorSubject = "agent:alice" },
		"delegator subject":       func(v *CanonicalBusinessIntent) { v.DelegatorSubject = "user:" },
		"agent subject":           func(v *CanonicalBusinessIntent) { v.AgentSubject = "user:bot" },
		"grant range":             func(v *CanonicalBusinessIntent) { v.DelegationGrantID = 0 },
		"padded command":          func(v *CanonicalBusinessIntent) { v.CommandID += "=" },
		"proof version downgrade": func(v *CanonicalBusinessIntent) { v.ProofVersion = DelegationProofVersion },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := valid
			mutate(&changed)
			if err := changed.Validate(); err == nil {
				t.Fatal("malformed intent was accepted")
			}
		})
	}
}

func TestCanonicalBusinessIntentRejectsStaleWitnessAndChangesHash(t *testing.T) {
	original := validCanonicalBusinessIntent(t)
	originalHash, err := original.IntentHash()
	if err != nil {
		t.Fatalf("hash original intent: %v", err)
	}

	stale := original
	stale.AmountMinor++
	if err := stale.Validate(); err == nil {
		t.Fatal("material change with stale witness was accepted")
	}

	changes := map[string]func(*CanonicalBusinessIntent){
		"tenant":   func(v *CanonicalBusinessIntent) { v.TenantID = "tenant-b" },
		"resource": func(v *CanonicalBusinessIntent) { v.ResourceID++ },
		"vendor":   func(v *CanonicalBusinessIntent) { v.VendorID++ },
		"currency": func(v *CanonicalBusinessIntent) { v.CurrencyCode = "EUR" },
		"amount":   func(v *CanonicalBusinessIntent) { v.AmountMinor++ },
		"line": func(v *CanonicalBusinessIntent) {
			digest := sha256.Sum256([]byte("changed lines"))
			v.LineDigest = hex.EncodeToString(digest[:])
		},
		"state":   func(v *CanonicalBusinessIntent) { v.RecordState = "draft" },
		"command": func(v *CanonicalBusinessIntent) { v.CommandID = base64.RawURLEncoding.EncodeToString(make([]byte, 32)) },
	}
	for name, mutate := range changes {
		t.Run(name, func(t *testing.T) {
			changed := original
			mutate(&changed)
			if err := changed.RefreshStateWitness(); err != nil {
				t.Fatalf("refresh changed witness: %v", err)
			}
			changedHash, err := changed.IntentHash()
			if err != nil {
				t.Fatalf("hash changed intent: %v", err)
			}
			if changedHash == originalHash {
				t.Fatal("material change did not change intent hash")
			}
		})
	}
}
