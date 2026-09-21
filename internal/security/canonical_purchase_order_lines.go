package security

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const canonicalPurchaseOrderLinesDomain = "PDP-ODOO-PO-LINES-V1"

var canonicalDecimalPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]*[1-9])?$`)

// CanonicalPurchaseOrderLine contains the persisted fields that can affect PO
// confirmation semantics. The encoder sorts lines and tax IDs deterministically.
type CanonicalPurchaseOrderLine struct {
	LineID           int64   `json:"line_id"`
	Sequence         int64   `json:"sequence"`
	DisplayType      string  `json:"display_type"`
	ProductID        int64   `json:"product_id"`
	Description      string  `json:"description"`
	UOMID            int64   `json:"uom_id"`
	Quantity         string  `json:"quantity"`
	UnitPrice        string  `json:"unit_price"`
	TaxIDs           []int64 `json:"tax_ids"`
	PlannedAt        string  `json:"planned_at"`
	LineWriteVersion string  `json:"line_write_version"`
}

// CanonicalPurchaseOrderLinesBytes encodes lines sorted by (sequence, line_id).
func CanonicalPurchaseOrderLinesBytes(lines []CanonicalPurchaseOrderLine) ([]byte, error) {
	if len(lines) > math.MaxUint32 {
		return nil, errors.New("purchase order line count exceeds uint32")
	}
	ordered := append([]CanonicalPurchaseOrderLine(nil), lines...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Sequence == ordered[j].Sequence {
			return ordered[i].LineID < ordered[j].LineID
		}
		return ordered[i].Sequence < ordered[j].Sequence
	})

	var payload bytes.Buffer
	payload.WriteString(canonicalPurchaseOrderLinesDomain)
	writeCanonicalUint32(&payload, uint32(len(ordered)))
	seenLines := make(map[int64]struct{}, len(ordered))
	for _, line := range ordered {
		if _, exists := seenLines[line.LineID]; exists {
			return nil, fmt.Errorf("duplicate purchase order line ID %d", line.LineID)
		}
		seenLines[line.LineID] = struct{}{}
		if err := writeCanonicalPurchaseOrderLine(&payload, line); err != nil {
			return nil, err
		}
	}
	return payload.Bytes(), nil
}

// CanonicalPurchaseOrderLineDigest returns SHA-256 over canonical line bytes.
func CanonicalPurchaseOrderLineDigest(lines []CanonicalPurchaseOrderLine) ([sha256.Size]byte, error) {
	payload, err := CanonicalPurchaseOrderLinesBytes(lines)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(payload), nil
}

func writeCanonicalPurchaseOrderLine(payload *bytes.Buffer, line CanonicalPurchaseOrderLine) error {
	description, taxes, err := validateCanonicalPurchaseOrderLine(line)
	if err != nil {
		return err
	}
	writeCanonicalInt64(payload, line.LineID)
	writeCanonicalInt64(payload, line.Sequence)
	writeCanonicalString(payload, line.DisplayType)
	writeCanonicalInt64(payload, line.ProductID)
	writeCanonicalString(payload, description)
	writeCanonicalInt64(payload, line.UOMID)
	writeCanonicalString(payload, line.Quantity)
	writeCanonicalString(payload, line.UnitPrice)
	writeCanonicalUint32(payload, uint32(len(taxes)))
	for _, taxID := range taxes {
		writeCanonicalInt64(payload, taxID)
	}
	writeCanonicalString(payload, line.PlannedAt)
	writeCanonicalString(payload, line.LineWriteVersion)
	return nil
}

func validateCanonicalPurchaseOrderLine(line CanonicalPurchaseOrderLine) (string, []int64, error) {
	if line.LineID <= 0 {
		return "", nil, errors.New("line_id must be positive")
	}
	displayLine := line.DisplayType == "line_section" || line.DisplayType == "line_note"
	if line.DisplayType != "" && !displayLine {
		return "", nil, errors.New("unsupported display_type")
	}
	if (!displayLine && (line.ProductID <= 0 || line.UOMID <= 0)) ||
		(displayLine && (line.ProductID != 0 || line.UOMID != 0)) {
		return "", nil, errors.New("product_id and uom_id do not match display_type")
	}
	if !utf8.ValidString(line.Description) {
		return "", nil, errors.New("description must be non-empty canonical UTF-8")
	}
	description := norm.NFC.String(strings.ReplaceAll(strings.ReplaceAll(line.Description, "\r\n", "\n"), "\r", "\n"))
	if description == "" || uint64(len(description)) > uint64(math.MaxUint32) {
		return "", nil, errors.New("description must be non-empty canonical UTF-8")
	}
	if err := validateCanonicalDecimal("quantity", line.Quantity); err != nil {
		return "", nil, err
	}
	if err := validateCanonicalDecimal("unit_price", line.UnitPrice); err != nil {
		return "", nil, err
	}
	taxes := append([]int64(nil), line.TaxIDs...)
	sort.Slice(taxes, func(i, j int) bool { return taxes[i] < taxes[j] })
	for index, taxID := range taxes {
		if taxID <= 0 || (index > 0 && taxes[index-1] == taxID) {
			return "", nil, errors.New("tax_ids must contain unique positive IDs")
		}
	}
	if line.PlannedAt == "" {
		if !displayLine {
			return "", nil, errors.New("planned_at is required for product lines")
		}
	} else if err := validateCanonicalTimestamp("planned_at", line.PlannedAt); err != nil {
		return "", nil, err
	}
	if err := validateCanonicalTimestamp("line_write_version", line.LineWriteVersion); err != nil {
		return "", nil, err
	}
	return description, taxes, nil
}

func validateCanonicalDecimal(field, value string) error {
	if !canonicalDecimalPattern.MatchString(value) || value == "-0" {
		return fmt.Errorf("%s must be a canonical decimal", field)
	}
	return nil
}
