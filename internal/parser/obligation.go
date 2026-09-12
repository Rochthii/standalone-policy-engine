package parser

import "fmt"

const (
	maxPolicyObligations    = 4
	maxObligationMessageLen = 256
)

func (p *Parser) parseObligation() *ObligationNode {
	position := p.curToken.Pos
	if !p.expectPeek(TokIdent) {
		return nil
	}
	obligation := &ObligationNode{
		Type: ObligationType(p.curToken.Literal),
		pos:  position,
	}
	if p.peekTokenIs(TokString) {
		p.nextToken()
		obligation.Message = p.curToken.Literal
	}
	return obligation
}

func validatePolicyObligations(policy *PolicyNode) error {
	if len(policy.Obligations) > maxPolicyObligations {
		return fmt.Errorf("policy may define at most %d obligations", maxPolicyObligations)
	}
	seen := make(map[ObligationType]struct{}, len(policy.Obligations))
	for _, obligation := range policy.Obligations {
		switch obligation.Type {
		case ObligationRequireHumanApproval, ObligationAuditSensitive, ObligationMaskAttributes:
		default:
			return fmt.Errorf("unsupported obligation type %q", obligation.Type)
		}
		if _, duplicate := seen[obligation.Type]; duplicate {
			return fmt.Errorf("duplicate obligation type %q", obligation.Type)
		}
		seen[obligation.Type] = struct{}{}
		if len(obligation.Message) > maxObligationMessageLen {
			return fmt.Errorf("obligation %q message exceeds %d bytes", obligation.Type, maxObligationMessageLen)
		}
	}
	return nil
}
