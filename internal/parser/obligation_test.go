package parser

import (
	"strings"
	"testing"
)

func TestCompilePolicySetParsesTypedObligation(t *testing.T) {
	policies, err := CompilePolicySet([]PolicySource{{
		ID: "high-value-deny",
		Text: `forbid(principal == any, action == action:APPROVE, resource == any)
when { context.amount > 2000 }
obligation REQUIRE_HUMAN_APPROVAL "Human approval is required";`,
	}})
	if err != nil {
		t.Fatalf("compile policy with obligation: %v", err)
	}
	if len(policies) != 1 || len(policies[0].Obligations) != 1 {
		t.Fatalf("compiled obligation missing: %#v", policies)
	}
	obligation := policies[0].Obligations[0]
	if obligation.Type != ObligationRequireHumanApproval || obligation.Message != "Human approval is required" {
		t.Fatalf("unexpected typed obligation: %#v", obligation)
	}
}

func TestCompilerRejectsInvalidPolicyObligations(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "unsupported",
			text: `forbid(principal == any, action == any, resource == any)
obligation RUN_ARBITRARY_TOOL "unsafe";`,
			want: "unsupported obligation",
		},
		{
			name: "duplicate",
			text: `forbid(principal == any, action == any, resource == any)
obligation REQUIRE_HUMAN_APPROVAL "first"
obligation REQUIRE_HUMAN_APPROVAL "second";`,
			want: "duplicate obligation",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CompilePolicySet([]PolicySource{{ID: test.name, Text: test.text}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}
