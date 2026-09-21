package security

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

type cbiGoldenFixture struct {
	KeyID       string                       `json:"key_id"`
	Secret      string                       `json:"secret"`
	IssuedAt    int64                        `json:"issued_at"`
	ValidUntil  int64                        `json:"valid_until"`
	AmountTotal string                       `json:"amount_total"`
	Intent      CanonicalBusinessIntent      `json:"intent"`
	Lines       []CanonicalPurchaseOrderLine `json:"lines"`
	Expected    struct {
		LineBytesBase64   string `json:"line_bytes_base64"`
		LineDigest        string `json:"line_digest"`
		StateWitness      string `json:"state_witness"`
		IntentBytesBase64 string `json:"intent_bytes_base64"`
		IntentHash        string `json:"intent_hash"`
		Proof             string `json:"proof"`
	} `json:"expected"`
}

func loadCBIGoldenFixture(t *testing.T) cbiGoldenFixture {
	t.Helper()
	file, err := os.Open("testdata/cbi_v1_golden.json")
	if err != nil {
		t.Fatalf("open CBI golden fixture: %v", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var fixture cbiGoldenFixture
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatalf("decode CBI golden fixture: %v", err)
	}
	return fixture
}

func buildGoldenIntent(t *testing.T, fixture cbiGoldenFixture) CanonicalBusinessIntent {
	t.Helper()
	digest, err := CanonicalPurchaseOrderLineDigest(fixture.Lines)
	if err != nil {
		t.Fatalf("digest canonical lines: %v", err)
	}
	intent := fixture.Intent
	intent.LineDigest = hex.EncodeToString(digest[:])
	if err := intent.RefreshStateWitness(); err != nil {
		t.Fatalf("refresh golden state witness: %v", err)
	}
	return intent
}

func TestCBIV2CrossLanguageGoldenVector(t *testing.T) {
	fixture := loadCBIGoldenFixture(t)
	lineBytes, err := CanonicalPurchaseOrderLinesBytes(fixture.Lines)
	if err != nil {
		t.Fatalf("encode canonical lines: %v", err)
	}
	if got := base64.StdEncoding.EncodeToString(lineBytes); got != fixture.Expected.LineBytesBase64 {
		t.Fatalf("line bytes mismatch: got %s", got)
	}
	intent := buildGoldenIntent(t, fixture)
	if intent.LineDigest != fixture.Expected.LineDigest || intent.StateWitness != fixture.Expected.StateWitness {
		t.Fatal("line digest or state witness does not match cross-language fixture")
	}
	intentBytes, err := intent.CanonicalBytes()
	if err != nil {
		t.Fatalf("encode golden intent: %v", err)
	}
	if got := base64.StdEncoding.EncodeToString(intentBytes); got != fixture.Expected.IntentBytesBase64 {
		t.Fatalf("intent bytes mismatch: got %s", got)
	}
	intentHash, err := intent.IntentHash()
	if err != nil {
		t.Fatalf("hash golden intent: %v", err)
	}
	if hex.EncodeToString(intentHash[:]) != fixture.Expected.IntentHash {
		t.Fatalf("intent hash mismatch: got %x", intentHash)
	}
	manager, err := NewDelegationManagerWithKeyring(fixture.KeyID, map[string]string{fixture.KeyID: fixture.Secret})
	if err != nil {
		t.Fatalf("create golden key ring: %v", err)
	}
	proof, err := manager.GenerateProofV2(DelegationProofV2Input{
		Intent: intent, IssuedAt: fixture.IssuedAt, ValidUntil: fixture.ValidUntil,
	})
	if err != nil {
		t.Fatalf("generate golden V2 proof: %v", err)
	}
	if proof != fixture.Expected.Proof {
		t.Fatalf("V2 proof mismatch: got %s", proof)
	}
}

func TestCanonicalPurchaseOrderLinesSortAndValidate(t *testing.T) {
	fixture := loadCBIGoldenFixture(t)
	original, err := CanonicalPurchaseOrderLinesBytes(fixture.Lines)
	if err != nil {
		t.Fatalf("encode original lines: %v", err)
	}
	reversed := cloneCanonicalLines(fixture.Lines)
	reversed[0], reversed[1] = reversed[1], reversed[0]
	reordered, err := CanonicalPurchaseOrderLinesBytes(reversed)
	if err != nil {
		t.Fatalf("encode reversed input: %v", err)
	}
	if string(original) != string(reordered) {
		t.Fatal("input slice order changed canonical line bytes")
	}
	invalid := cloneCanonicalLines(fixture.Lines)
	invalid[0].Quantity = "2.50"
	if _, err := CanonicalPurchaseOrderLinesBytes(invalid); err == nil {
		t.Fatal("non-canonical decimal was accepted")
	}
}

func TestDelegationProofV2RejectsMaterialTamperMatrix(t *testing.T) {
	fixture := loadCBIGoldenFixture(t)
	manager, err := NewDelegationManagerWithKeyring(fixture.KeyID, map[string]string{fixture.KeyID: fixture.Secret})
	if err != nil {
		t.Fatalf("create tamper key ring: %v", err)
	}
	now := time.Now().Unix()
	input := DelegationProofV2Input{Intent: buildGoldenIntent(t, fixture), IssuedAt: now, ValidUntil: now + 3600}
	proof, err := manager.GenerateProofV2(input)
	if err != nil {
		t.Fatalf("generate original V2 proof: %v", err)
	}

	intentCases := map[string]func(*CanonicalBusinessIntent){
		"amount":         func(v *CanonicalBusinessIntent) { v.AmountMinor++ },
		"currency":       func(v *CanonicalBusinessIntent) { v.CurrencyCode = "EUR" },
		"vendor":         func(v *CanonicalBusinessIntent) { v.VendorID++ },
		"resource":       func(v *CanonicalBusinessIntent) { v.ResourceID++ },
		"state":          func(v *CanonicalBusinessIntent) { v.RecordState = "draft" },
		"record version": func(v *CanonicalBusinessIntent) { v.RecordWriteVersion = "2026-09-20T10:11:12.123457Z" },
	}
	for name, mutate := range intentCases {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed.Intent)
			if err := changed.Intent.RefreshStateWitness(); err != nil {
				t.Fatalf("refresh changed intent: %v", err)
			}
			if err := manager.VerifyProofV2(changed, proof); err == nil {
				t.Fatal("old proof accepted changed material intent")
			}
		})
	}
	changedAction := input
	changedAction.Intent.Action = "action:CANCEL"
	if err := manager.VerifyProofV2(changedAction, proof); err == nil {
		t.Fatal("old proof accepted changed action")
	}

	lineCases := map[string]func([]CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine{
		"add": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			added := lines[0]
			added.LineID = 103
			added.Sequence = 20
			return append(lines, added)
		},
		"remove": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine { return lines[:1] },
		"sequence": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].Sequence++
			return lines
		},
		"product": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].ProductID++
			return lines
		},
		"description": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].Description += " changed"
			return lines
		},
		"uom": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine { lines[0].UOMID++; return lines },
		"quantity": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].Quantity = "3"
			return lines
		},
		"unit price": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].UnitPrice = "123.457"
			return lines
		},
		"tax": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].TaxIDs = []int64{3}
			return lines
		},
		"planned at": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].PlannedAt = "2026-09-25T08:30:00.000001Z"
			return lines
		},
		"line version": func(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
			lines[0].LineWriteVersion = "2026-09-20T10:10:00.000002Z"
			return lines
		},
	}
	for name, mutate := range lineCases {
		t.Run("line "+name, func(t *testing.T) {
			lines := mutate(cloneCanonicalLines(fixture.Lines))
			digest, err := CanonicalPurchaseOrderLineDigest(lines)
			if err != nil {
				t.Fatalf("digest changed lines: %v", err)
			}
			changed := input
			changed.Intent.LineDigest = hex.EncodeToString(digest[:])
			if err := changed.Intent.RefreshStateWitness(); err != nil {
				t.Fatalf("refresh line-changed intent: %v", err)
			}
			if err := manager.VerifyProofV2(changed, proof); err == nil {
				t.Fatal("old proof accepted changed purchase order lines")
			}
		})
	}
}

func cloneCanonicalLines(lines []CanonicalPurchaseOrderLine) []CanonicalPurchaseOrderLine {
	cloned := append([]CanonicalPurchaseOrderLine(nil), lines...)
	for index := range cloned {
		cloned[index].TaxIDs = append([]int64(nil), cloned[index].TaxIDs...)
	}
	return cloned
}
