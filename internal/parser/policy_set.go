package parser

import (
	"fmt"
	"strings"
)

// PolicySource is one persisted policy document and its stable identifier.
type PolicySource struct {
	ID   string
	Text string
}

// CompilePolicySet compiles a complete ruleset atomically. Any invalid source
// returns no compiled policies so callers cannot publish a partial allow set.
func CompilePolicySet(sources []PolicySource) ([]*PolicyNode, error) {
	compiledPolicies := make([]*PolicyNode, 0, len(sources))
	for _, source := range sources {
		lexer := NewLexer(source.Text)
		policyParser := NewParser(lexer)
		nodes := policyParser.Parse()
		if parseErrors := policyParser.Errors(); len(parseErrors) > 0 {
			return nil, fmt.Errorf("policy %s parse failed: %s", source.ID, strings.Join(parseErrors, "; "))
		}
		if len(nodes) != 1 {
			return nil, fmt.Errorf("policy %s must contain exactly one statement, got %d", source.ID, len(nodes))
		}
		nodes[0].ID = source.ID
		compiled, err := NewCompiler().Compile(nodes[0])
		if err != nil {
			return nil, fmt.Errorf("policy %s compile failed: %w", source.ID, err)
		}
		compiledPolicies = append(compiledPolicies, compiled)
	}
	return compiledPolicies, nil
}
