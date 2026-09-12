package parser

import "testing"

func TestCompilePolicySetIsAllOrNothing(t *testing.T) {
	valid := PolicySource{ID: "permit-read", Text: `permit(principal == any, action == action:READ, resource == any);`}
	invalid := PolicySource{ID: "broken-forbid", Text: `forbid(principal == any, action == action:DELETE, resource == any) when {`}

	compiled, err := CompilePolicySet([]PolicySource{valid, invalid})
	if err == nil {
		t.Fatal("expected invalid ruleset to fail")
	}
	if compiled != nil {
		t.Fatalf("partial ruleset must not be returned, got %d policies", len(compiled))
	}
}

func TestCompilePolicySetRequiresOneStatementPerSource(t *testing.T) {
	source := PolicySource{
		ID: "two-statements",
		Text: `permit(principal == any, action == action:READ, resource == any);
permit(principal == any, action == action:WRITE, resource == any);`,
	}
	if compiled, err := CompilePolicySet([]PolicySource{source}); err == nil || compiled != nil {
		t.Fatalf("expected multi-statement source to fail atomically, compiled=%v err=%v", compiled, err)
	}
}

func TestCompilePolicySetPreservesStableIDs(t *testing.T) {
	sources := []PolicySource{
		{ID: "permit-read", Text: `permit(principal == any, action == action:READ, resource == any);`},
		{ID: "forbid-delete", Text: `forbid(principal == any, action == action:DELETE, resource == any);`},
	}
	compiled, err := CompilePolicySet(sources)
	if err != nil {
		t.Fatalf("compile valid ruleset: %v", err)
	}
	if len(compiled) != 2 || compiled[0].ID != "permit-read" || compiled[1].ID != "forbid-delete" {
		t.Fatalf("unexpected compiled policy IDs: %#v", compiled)
	}
}
