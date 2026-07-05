package printer_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"
)

// captureStdout captures anything written to os.Stdout during fn execution.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintPoliciesTable(t *testing.T) {
	policies := []pectl.Policy{
		{
			ID:        "policy-1",
			Effect:    "permit",
			Status:    "ACTIVE",
			Version:   2,
			UpdatedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		},
	}

	output := captureStdout(func() {
		printer.PrintPoliciesTable(policies)
	})

	if !strings.Contains(output, "policy-1") {
		t.Errorf("expected 'policy-1' in output, got: %s", output)
	}
	if !strings.Contains(output, "ACTIVE") {
		t.Errorf("expected 'ACTIVE' in output, got: %s", output)
	}
	if !strings.Contains(output, "permit") {
		t.Errorf("expected 'permit' in output, got: %s", output)
	}
}

func TestPrintSimulationTable(t *testing.T) {
	res := &pectl.SimulateResp{
		SimulatedDecision: "ALLOW",
		Reason:            "Matched policy p1",
		TenantID:          "tenant-abc",
		MatchedPolicies:   []string{"p1"},
		CompileErrors:     []string{},
	}

	output := captureStdout(func() {
		printer.PrintSimulationTable(res)
	})

	if !strings.Contains(output, "ALLOW") {
		t.Errorf("expected 'ALLOW' in output, got: %s", output)
	}
	if !strings.Contains(output, "tenant-abc") {
		t.Errorf("expected 'tenant-abc' in output, got: %s", output)
	}
}

func TestPrintDecisionTable(t *testing.T) {
	res := &pectl.DecisionResp{
		Decision:        "DENY",
		MatchedPolicyID: "",
		EvaluatedAt:     time.Now(),
	}

	output := captureStdout(func() {
		printer.PrintDecisionTable(res)
	})

	if !strings.Contains(output, "DENY") {
		t.Errorf("expected 'DENY' in output, got: %s", output)
	}
}

func TestPrintMetricsTable(t *testing.T) {
	metrics := &pectl.MetricsResp{
		DecisionRate: 1234.56,
		QPS:          987.65,
		LatencyP50:   0.5,
		LatencyP95:   2.3,
		LatencyP99:   5.7,
		MemoryUsage:  1073741824,
		GCStatistics: "pause: 2ms",
	}

	output := captureStdout(func() {
		printer.PrintMetricsTable(metrics)
	})

	if !strings.Contains(output, "1234") {
		t.Errorf("expected decision rate in output, got: %s", output)
	}
	if !strings.Contains(output, "pause: 2ms") {
		t.Errorf("expected GC stats in output, got: %s", output)
	}
}

func TestPrintHealthTable(t *testing.T) {
	health := &pectl.HealthResp{
		Status: "UP",
		Components: map[string]pectl.HealthComponent{
			"database": {Status: "UP", Message: "connected"},
			"compiler": {Status: "UP"},
		},
	}

	output := captureStdout(func() {
		printer.PrintHealthTable(health)
	})

	if !strings.Contains(output, "UP") {
		t.Errorf("expected 'UP' in output, got: %s", output)
	}
	if !strings.Contains(output, "database") {
		t.Errorf("expected 'database' component in output, got: %s", output)
	}
}

func TestPrintJSON(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	output := captureStdout(func() {
		_ = printer.PrintJSON(data)
	})

	if !strings.Contains(output, `"key"`) {
		t.Errorf("expected JSON key in output, got: %s", output)
	}
	if !strings.Contains(output, `"value"`) {
		t.Errorf("expected JSON value in output, got: %s", output)
	}
}

func TestPrintYAML(t *testing.T) {
	data := map[string]interface{}{
		"server": "http://localhost:8080",
		"output": "table",
	}

	output := captureStdout(func() {
		_ = printer.PrintYAML(data)
	})

	if !strings.Contains(output, "server") {
		t.Errorf("expected 'server' in YAML output, got: %s", output)
	}
}
