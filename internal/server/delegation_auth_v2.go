package server

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"
)

const canonicalIntentContextPrefix = "cbi."

var canonicalIntentFields = []string{
	"intent_version", "tenant_id", "company_id", "resource_type", "resource_id",
	"action", "vendor_id", "currency_code", "currency_scale", "amount_minor",
	"line_digest", "record_state", "record_write_version", "state_witness",
	"creator_subject", "delegation_grant_id", "delegator_subject", "agent_subject",
	"command_id", "proof_version",
}

var canonicalIntentContextKeys = func() []string {
	keys := make([]string, len(canonicalIntentFields))
	for index, field := range canonicalIntentFields {
		keys[index] = canonicalIntentContextPrefix + field
	}
	return keys
}()

func requiresDelegationProofV2(req *policyv1.CheckAccessRequest) bool {
	if req == nil {
		return false
	}
	if req.Action == security.ConfirmPurchaseOrderAction {
		return true
	}
	const resourcePrefix = "purchase_order:"
	if !strings.HasPrefix(req.Resource, resourcePrefix) {
		return false
	}
	resourceID := strings.TrimPrefix(req.Resource, resourcePrefix)
	_, err := parseCanonicalInt64("resource_id", resourceID, true)
	return err == nil
}

func (s *GRPCServer) validateDelegationV2(req *policyv1.CheckAccessRequest, proof string) error {
	input, err := delegationProofV2Input(req)
	if err != nil {
		return err
	}
	if err := s.delegationMgr.VerifyProofV2(input, proof); err != nil {
		return err
	}
	bindCanonicalIntentContext(req, input.Intent)
	return nil
}

func delegationProofV2Input(req *policyv1.CheckAccessRequest) (security.DelegationProofV2Input, error) {
	if req == nil || req.Context == nil {
		return security.DelegationProofV2Input{}, errors.New("missing request context")
	}
	if err := rejectUnknownCanonicalIntentFields(req.Context); err != nil {
		return security.DelegationProofV2Input{}, err
	}
	companyID, err := canonicalContextInt64(req.Context, "company_id", true)
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	resourceID, err := canonicalContextInt64(req.Context, "resource_id", true)
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	vendorID, err := canonicalContextInt64(req.Context, "vendor_id", true)
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	currencyScale, err := canonicalContextInt64(req.Context, "currency_scale", false)
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	amountMinor, err := canonicalContextInt64(req.Context, "amount_minor", false)
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	grantID, err := canonicalContextInt64(req.Context, "delegation_grant_id", true)
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}

	intent := security.CanonicalBusinessIntent{
		IntentVersion:      canonicalContext(req.Context, "intent_version"),
		TenantID:           canonicalContext(req.Context, "tenant_id"),
		CompanyID:          companyID,
		ResourceType:       canonicalContext(req.Context, "resource_type"),
		ResourceID:         resourceID,
		Action:             canonicalContext(req.Context, "action"),
		VendorID:           vendorID,
		CurrencyCode:       canonicalContext(req.Context, "currency_code"),
		CurrencyScale:      currencyScale,
		AmountMinor:        amountMinor,
		LineDigest:         canonicalContext(req.Context, "line_digest"),
		RecordState:        canonicalContext(req.Context, "record_state"),
		RecordWriteVersion: canonicalContext(req.Context, "record_write_version"),
		StateWitness:       canonicalContext(req.Context, "state_witness"),
		CreatorSubject:     canonicalContext(req.Context, "creator_subject"),
		DelegationGrantID:  grantID,
		DelegatorSubject:   canonicalContext(req.Context, "delegator_subject"),
		AgentSubject:       canonicalContext(req.Context, "agent_subject"),
		CommandID:          canonicalContext(req.Context, "command_id"),
		ProofVersion:       canonicalContext(req.Context, "proof_version"),
	}
	if err := intent.Validate(); err != nil {
		return security.DelegationProofV2Input{}, err
	}
	if err := validateIntentRequestBinding(req, intent); err != nil {
		return security.DelegationProofV2Input{}, err
	}
	issuedAt, err := parseDelegationTimestamp(req.Context["delegation_issued_at"], "delegation_issued_at")
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	validUntil, err := parseDelegationTimestamp(req.Context["delegation_valid_until"], "delegation_valid_until")
	if err != nil {
		return security.DelegationProofV2Input{}, err
	}
	return security.DelegationProofV2Input{Intent: intent, IssuedAt: issuedAt, ValidUntil: validUntil}, nil
}

func validateIntentRequestBinding(req *policyv1.CheckAccessRequest, intent security.CanonicalBusinessIntent) error {
	expectedResource := "purchase_order:" + strconv.FormatInt(intent.ResourceID, 10)
	expectedGrant := strconv.FormatInt(intent.DelegationGrantID, 10)
	expectedChain := intent.DelegatorSubject + "," + intent.AgentSubject
	checks := []struct{ name, actual, expected string }{
		{"tenant", req.TenantId, intent.TenantID},
		{"subject", req.Subject, intent.AgentSubject},
		{"action", req.Action, intent.Action},
		{"resource", req.Resource, expectedResource},
		{"grant", req.Context["delegation_grant_id"], expectedGrant},
		{"delegator", req.Context["delegated_by"], intent.DelegatorSubject},
		{"creator", req.Context["resource.creator_id"], intent.CreatorSubject},
		{"command", req.Context["delegation_nonce"], intent.CommandID},
		{"chain", req.Context["delegation_chain"], expectedChain},
	}
	for _, check := range checks {
		if check.actual != check.expected {
			return fmt.Errorf("%s does not match canonical business intent", check.name)
		}
	}
	return nil
}

func bindCanonicalIntentContext(req *policyv1.CheckAccessRequest, intent security.CanonicalBusinessIntent) {
	req.Context["amount"] = canonicalMajorAmount(intent.AmountMinor, intent.CurrencyScale)
	req.Context["amount_minor"] = strconv.FormatInt(intent.AmountMinor, 10)
	req.Context["currency"] = intent.CurrencyCode
	req.Context["resource.creator_id"] = intent.CreatorSubject
	req.Context["delegated_by"] = intent.DelegatorSubject
	req.Context["delegation_chain"] = intent.DelegatorSubject + "," + intent.AgentSubject
	req.Context["tool_context"] = "tool:auto_confirm_po"
	req.Context["execution_mode"] = "autonomous_run"
}

func canonicalMajorAmount(amountMinor, scale int64) string {
	if scale == 0 {
		return strconv.FormatInt(amountMinor, 10)
	}
	factor := int64(1)
	for range scale {
		factor *= 10
	}
	whole, fraction := amountMinor/factor, amountMinor%factor
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	fractionText := fmt.Sprintf("%0*d", int(scale), fraction)
	return strconv.FormatInt(whole, 10) + "." + strings.TrimRight(fractionText, "0")
}

func canonicalContext(context map[string]string, field string) string {
	return context[canonicalIntentContextPrefix+field]
}

func canonicalContextInt64(context map[string]string, field string, positive bool) (int64, error) {
	return parseCanonicalInt64(field, canonicalContext(context, field), positive)
}

func parseCanonicalInt64(field, value string, positive bool) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || strconv.FormatInt(parsed, 10) != value || (positive && parsed <= 0) || (!positive && parsed < 0) {
		return 0, fmt.Errorf("%s is not a canonical integer", field)
	}
	return parsed, nil
}

func rejectUnknownCanonicalIntentFields(context map[string]string) error {
	allowed := make(map[string]struct{}, len(canonicalIntentContextKeys))
	for _, key := range canonicalIntentContextKeys {
		allowed[key] = struct{}{}
	}
	for key := range context {
		if strings.HasPrefix(key, canonicalIntentContextPrefix) {
			if _, exists := allowed[key]; !exists {
				return fmt.Errorf("unknown canonical business intent field %s", key)
			}
		}
	}
	return nil
}
