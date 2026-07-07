package engine

import (
	"testing"

	"standalone-policy-engine/internal/parser"
)

// --- TestTrie_AddAndLookup_BasicFlow ---

// Kiểm tra luồng cơ bản: thêm policy vào Trie và tra cứu thành công.
func TestTrie_AddAndLookup_BasicFlow(t *testing.T) {
	trie := NewTrieRoot("tenant-trie-basic")

	c := parser.NewCompiler()
	l := parser.NewLexer(`permit(principal == user:"alice", action == action:READ, resource == file:"report.pdf")
when { context.ip_address in "192.168.0.0/16" };`)
	p := parser.NewParser(l)
	nodes := p.Parse()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse error: %v", p.Errors())
	}
	nodes[0].ID = "P-BASIC"
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	trie.AddPolicy(compiled)

	// Lookup với subject, resource, action khớp chính xác
	results := trie.LookupPolicies(
		[]string{"user:alice"},
		[]string{"file:report.pdf"},
		"action:READ",
	)

	if len(results) == 0 {
		t.Fatal("LookupPolicies: mong đợi ít nhất 1 policy, kết quả rỗng")
	}

	found := false
	for _, pol := range results {
		if pol.ID == "P-BASIC" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("LookupPolicies: không tìm thấy P-BASIC trong kết quả %v", results)
	}
}

// --- TestTrie_GlobalPolicySeparation ---

// Kiểm tra phân vùng Global Rules: chính sách principal==any AND resource==any
// phải được lưu vào GlobalPolicies, không vào Subjects trie.
func TestTrie_GlobalPolicySeparation(t *testing.T) {
	trie := NewTrieRoot("tenant-trie-global")

	c := parser.NewCompiler()

	// Policy toàn cục: cả principal và resource đều là any
	globalDSL := `forbid(principal == any, action == any, resource == any)
when { context.device_status == "compromised" };`
	l := parser.NewLexer(globalDSL)
	p := parser.NewParser(l)
	nodes := p.Parse()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse error: %v", p.Errors())
	}
	nodes[0].ID = "P-GLOBAL"
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	trie.AddPolicy(compiled)

	// Phải nằm trong GlobalPolicies
	if len(trie.GlobalPolicies) != 1 || trie.GlobalPolicies[0].ID != "P-GLOBAL" {
		t.Errorf("GlobalPolicies: mong đợi P-GLOBAL, thực tế: %v", trie.GlobalPolicies)
	}

	// Phải KHÔNG nằm trong Subjects trie
	if len(trie.Subjects) != 0 {
		t.Errorf("Subjects: chính sách toàn cục không được đưa vào Subjects trie, thực tế len=%d", len(trie.Subjects))
	}

	// LookupPolicies phải tự động bao gồm GlobalPolicies
	results := trie.LookupPolicies(
		[]string{"user:anyone"},
		[]string{"file:anything"},
		"action:DELETE",
	)

	found := false
	for _, pol := range results {
		if pol.ID == "P-GLOBAL" {
			found = true
			break
		}
	}
	if !found {
		t.Error("LookupPolicies: GlobalPolicies phải luôn được bao gồm trong kết quả lookup")
	}
}

// --- TestTrie_WildcardActionMatching ---

// Chính sách action==any phải khớp với bất kỳ action nào khi lookup.
func TestTrie_WildcardActionMatching(t *testing.T) {
	trie := NewTrieRoot("tenant-trie-wildcard-action")

	c := parser.NewCompiler()

	// Policy: principal cụ thể, resource cụ thể, action là any (wildcard)
	dsl := `permit(principal == user:"admin", action == any, resource == file:"sensitive")
when { context.ip_address in "10.0.0.0/8" };`
	l := parser.NewLexer(dsl)
	p := parser.NewParser(l)
	nodes := p.Parse()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse error: %v", p.Errors())
	}
	nodes[0].ID = "P-WILDCARD-ACTION"
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	trie.AddPolicy(compiled)

	// Lookup với các action khác nhau — tất cả phải tìm thấy P-WILDCARD-ACTION
	for _, action := range []string{"action:READ", "action:WRITE", "action:DELETE", "action:EXPORT"} {
		results := trie.LookupPolicies(
			[]string{"user:admin"},
			[]string{"file:sensitive"},
			action,
		)
		found := false
		for _, pol := range results {
			if pol.ID == "P-WILDCARD-ACTION" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("action=%s: mong đợi tìm thấy P-WILDCARD-ACTION qua wildcard, kết quả: %v", action, results)
		}
	}
}

// --- TestTrie_TenantIsolation ---

// Policy của tenant A không được leak sang tenant B.
// Mỗi TrieRoot hoàn toàn độc lập.
func TestTrie_TenantIsolation(t *testing.T) {
	trieA := NewTrieRoot("tenant-A")
	trieB := NewTrieRoot("tenant-B")

	c := parser.NewCompiler()

	// Thêm policy vào tenant A
	dslA := `permit(principal == user:"alice", action == action:READ, resource == file:"doc-a")
when { context.ip_address in "10.0.0.0/8" };`
	lA := parser.NewLexer(dslA)
	pA := parser.NewParser(lA)
	nodesA := pA.Parse()
	if len(pA.Errors()) > 0 {
		t.Fatalf("parse error tenant A: %v", pA.Errors())
	}
	nodesA[0].ID = "P-TENANT-A"
	compiledA, err := c.Compile(nodesA[0])
	if err != nil {
		t.Fatalf("compile error tenant A: %v", err)
	}
	trieA.AddPolicy(compiledA)

	// Lookup từ trieB — không được thấy policy của tenant A
	resultsFromB := trieB.LookupPolicies(
		[]string{"user:alice"},
		[]string{"file:doc-a"},
		"action:READ",
	)

	for _, pol := range resultsFromB {
		if pol.ID == "P-TENANT-A" {
			t.Error("Tenant isolation violation: Trie của tenant B chứa policy của tenant A")
		}
	}

	// Lookup từ trieA — phải thấy policy của tenant A
	resultsFromA := trieA.LookupPolicies(
		[]string{"user:alice"},
		[]string{"file:doc-a"},
		"action:READ",
	)

	found := false
	for _, pol := range resultsFromA {
		if pol.ID == "P-TENANT-A" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Trie của tenant A không tìm thấy chính sách của chính mình")
	}
}

// --- TestTrie_MultipleSubjectInheritanceLookup ---

// Khi lookup với danh sách subjects (kết quả kế thừa vai trò từ DAG),
// Trie phải tìm đúng policy khớp với bất kỳ subject nào trong danh sách.
func TestTrie_MultipleSubjectInheritanceLookup(t *testing.T) {
	trie := NewTrieRoot("tenant-trie-inherit")

	c := parser.NewCompiler()

	// Policy cho role:admin
	dslAdmin := `permit(principal == role:"admin", action == action:WRITE, resource == file:"config")
when { context.ip_address in "0.0.0.0/0" };`
	lAdmin := parser.NewLexer(dslAdmin)
	pAdmin := parser.NewParser(lAdmin)
	nodesAdmin := pAdmin.Parse()
	if len(pAdmin.Errors()) > 0 {
		t.Fatalf("parse error: %v", pAdmin.Errors())
	}
	nodesAdmin[0].ID = "P-ROLE-ADMIN"
	compiledAdmin, err := c.Compile(nodesAdmin[0])
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	trie.AddPolicy(compiledAdmin)

	// user:alice kế thừa role:admin (subjects = ["user:alice", "role:admin"])
	// Trie phải tìm thấy P-ROLE-ADMIN qua "role:admin" trong danh sách
	results := trie.LookupPolicies(
		[]string{"user:alice", "role:admin"},
		[]string{"file:config"},
		"action:WRITE",
	)

	found := false
	for _, pol := range results {
		if pol.ID == "P-ROLE-ADMIN" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Trie không tìm thấy policy qua role kế thừa trong danh sách subjects")
	}
}

// --- TestTrie_EmptyLookupNoPanic ---

// Lookup trên Trie rỗng hoặc với subject/resource không tồn tại
// không được panic — phải trả về slice rỗng.
func TestTrie_EmptyLookupNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("LookupPolicies panic trên Trie rỗng: %v", r)
		}
	}()

	trie := NewTrieRoot("tenant-trie-empty")

	results := trie.LookupPolicies(
		[]string{"user:nobody"},
		[]string{"file:nonexistent"},
		"action:READ",
	)

	if results == nil {
		t.Error("LookupPolicies phải trả về slice rỗng (không phải nil) khi không tìm thấy")
	}
}
