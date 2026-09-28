package security_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/security"
)

// Opt-in, single-worker per-invocation samples, NOT testing.B ns/op replicas.
// Keep production packages free of profiling hooks. Sign/setup before timing.
func TestEval03Latency(t *testing.T) {
	output := os.Getenv("PDP_EVAL03_GO_OUTPUT")
	if output == "" {
		t.Skip("set PDP_EVAL03_GO_OUTPUT to a new JSON path to collect samples")
	}
	count := eval03Count(t, "PDP_EVAL03_SAMPLES", 1000)
	warmup := eval03Count(t, "PDP_EVAL03_WARMUP", 100)
	clock := newEvalClock(t)
	eng := engine.NewEngine()
	syntax := parser.NewParser(parser.NewLexer(`permit(principal == any, action == action:CONFIRM_PURCHASE_ORDER, resource == any) when { context.amount <= 2000 };`))
	nodes := syntax.Parse()
	if len(syntax.Errors()) != 0 || len(nodes) != 1 {
		t.Fatalf("fixture parse: %v", syntax.Errors())
	}
	policy, err := parser.NewCompiler().Compile(nodes[0])
	if err != nil {
		t.Fatal(err)
	}
	policy.ID = "eval03-permit"
	if err := eng.UpdateTenantPolicies("tenant-a", []*parser.PolicyNode{policy}, nil); err != nil {
		t.Fatal(err)
	}
	request := map[string]string{"amount": "1000"}
	manager := security.NewDelegationManagerWithSecret("eval03-test-only-delegation-key-at-least-32-characters")
	input, approvalInput := eval03Fixture(t)
	proof, err := manager.GenerateProofV2(input)
	if err != nil {
		t.Fatal(err)
	}
	approvals, err := security.NewApprovalCapabilityManagerWithKeyring("eval03-approval",
		map[string]string{"eval03-approval": "eval03-test-only-approval-key-at-least-32-characters"}, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	capability, envelope, err := approvals.Issue(approvalInput, input.ValidUntil)
	if err != nil {
		t.Fatal(err)
	}
	ops := []struct {
		name string
		run  func() bool
	}{
		{"timer_control", func() bool { return true }},
		{"evaluator_single_permit", func() bool {
			return eng.CheckPermission(context.Background(), "tenant-a", input.Intent.AgentSubject,
				security.ConfirmPurchaseOrderAction, "purchase_order:42", request).Decision == engine.DecisionAllow
		}},
		{"proof_v2_verify", func() bool { return manager.VerifyProofV2(input, proof) == nil }},
		{"capability_v1_verify", func() bool { return approvals.Verify(capability, envelope) == nil }},
	}
	rows := make([]map[string]any, 0, len(ops)*(count+warmup))
	status := "PASS"
	started := time.Now().UTC().Format(time.RFC3339Nano)
	for _, phase := range []struct {
		name  string
		count int
	}{{"warmup", warmup}, {"sample", count}} {
		for i := 0; i < phase.count; i++ {
			// Rotate order to avoid timing every operation only in a single block.
			for offset := range ops {
				op := ops[(i+offset)%len(ops)]
				begin := clock.now()
				ok := op.run()
				elapsed := clock.elapsed(begin)
				rowStatus := "PASS"
				var errorType any
				if !ok {
					rowStatus, status, errorType = "FAIL", "FAIL", "unexpected_verification_or_decision"
				}
				// An empty control may fit within one clock tick; API calls must resolve.
				if elapsed < 0 || (elapsed == 0 && op.name != "timer_control") {
					rowStatus, status, errorType = "FAIL", "FAIL", "timer_not_resolved"
				}
				rows = append(rows, map[string]any{"phase": phase.name, "route": "go_micro", "index": i,
					"status": rowStatus, "events": []map[string]any{{"id": 1, "parent_id": nil,
						"boundary": op.name, "elapsed_ns": elapsed, "error_type": errorType}}})
			}
		}
	}
	kind := "measurement"
	if os.Getenv("PDP_EVAL03_SMOKE") == "1" {
		kind = "smoke"
	}
	result := map[string]any{
		"schema": "eval03.v1", "kind": kind, "status": status, "started_utc": started,
		"finished_utc": time.Now().UTC().Format(time.RFC3339Nano), "rows": rows,
		"planned_per_boundary": map[string]int{"warmup": warmup, "sample": count},
		"environment": map[string]any{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
			"cpu_count": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0),
			"git_commit": os.Getenv("PDP_GIT_COMMIT"), "timer": clock.metadata()},
		"fixture":       "single compiled permit, amount=1000; CBI golden intent and derived AC; real-clock validity; setup/signing excluded",
		"method":        "warm sequential per-call Go APIs, result predicate and timer overhead included; no DB/RPC; not ERP policy workload",
		"source_sha256": eval03Sources(t),
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("artifact write/close: %v / %v", writeErr, closeErr)
	}
	t.Log("EVAL03_RESULT=" + output)
	if status != "PASS" {
		t.Fatal("measurement contains errors; inspect raw artifact before any retry")
	}
}

func eval03Count(t *testing.T, key string, fallback int) int {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		t.Fatalf("%s must be a positive integer", key)
	}
	return count
}

func eval03Fixture(t *testing.T) (security.DelegationProofV2Input, security.ApprovalCapability) {
	t.Helper()
	data, err := os.ReadFile("testdata/cbi_v1_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Intent security.CanonicalBusinessIntent      `json:"intent"`
		Lines  []security.CanonicalPurchaseOrderLine `json:"lines"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	digest, err := security.CanonicalPurchaseOrderLineDigest(fixture.Lines)
	if err != nil {
		t.Fatal(err)
	}
	intent := fixture.Intent
	intent.LineDigest = hex.EncodeToString(digest[:])
	if err := intent.RefreshStateWitness(); err != nil {
		t.Fatal(err)
	}
	hash, err := intent.IntentHash()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	input := security.DelegationProofV2Input{Intent: intent, IssuedAt: now, ValidUntil: now + 3600}
	approval := security.ApprovalCapability{
		ApprovalID: base64.RawURLEncoding.EncodeToString([]byte("eval03-fixture01")),
		TenantID:   intent.TenantID, CompanyID: intent.CompanyID,
		IntentHash: hex.EncodeToString(hash[:]), StateWitness: intent.StateWitness,
		CommandID: intent.CommandID, DelegationGrantID: intent.DelegationGrantID,
		DelegatorSubject: intent.DelegatorSubject, AgentSubject: intent.AgentSubject,
		CreatorSubject: intent.CreatorSubject, ApproverUserID: 77, ApproverSubject: "user:eval03-approver",
		IssuancePolicyRevision: 9,
	}
	return input, approval
}

func eval03Sources(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..")
	hashes := map[string]string{}
	for _, dir := range []string{"internal", "go.mod", "go.sum"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || (filepath.Ext(path) != ".go" && filepath.Ext(path) != ".json" && entry.Name() != "go.mod" && entry.Name() != "go.sum") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(data)
			hashes[filepath.ToSlash(relative)] = hex.EncodeToString(digest[:])
			return nil
		})
		if err != nil {
			t.Fatal(fmt.Errorf("fingerprint: %w", err))
		}
	}
	return hashes
}
