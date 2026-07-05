package printer

import (
	"fmt"
	"os"
	"standalone-policy-engine/internal/pectl"
	"strings"
	"text/tabwriter"
	"time"
)

// PrintPoliciesTable writes policy details in tabular form to stdout.
func PrintPoliciesTable(policies []pectl.Policy) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "POLICY ID\tEFFECT\tSTATUS\tVERSION\tUPDATED AT")
	for _, p := range policies {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			p.ID,
			p.Effect,
			p.Status,
			p.Version,
			p.UpdatedAt.Format(time.RFC3339),
		)
	}
	_ = w.Flush()
}

// PrintPolicyDetailTable prints a detailed view of a policy.
func PrintPolicyDetailTable(p *pectl.Policy) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Field\tValue\n")
	_, _ = fmt.Fprintf(w, "Policy ID\t%s\n", p.ID)
	_, _ = fmt.Fprintf(w, "Tenant ID\t%s\n", p.TenantID)
	_, _ = fmt.Fprintf(w, "Effect\t%s\n", p.Effect)
	_, _ = fmt.Fprintf(w, "Status\t%s\n", p.Status)
	_, _ = fmt.Fprintf(w, "Version\t%d\n", p.Version)
	_, _ = fmt.Fprintf(w, "Created At\t%s\n", p.CreatedAt.Format(time.RFC3339))
	_, _ = fmt.Fprintf(w, "Updated At\t%s\n", p.UpdatedAt.Format(time.RFC3339))
	_, _ = fmt.Fprintf(w, "Policy Text\t%s\n", strings.ReplaceAll(p.PolicyText, "\n", " "))
	_ = w.Flush()
}

// PrintSimulationTable prints simulation results in tabular form.
func PrintSimulationTable(res *pectl.SimulateResp) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Simulated Decision:\t%s\n", res.SimulatedDecision)
	_, _ = fmt.Fprintf(w, "Reason:\t%s\n", res.Reason)
	_, _ = fmt.Fprintf(w, "Tenant ID:\t%s\n", res.TenantID)
	if len(res.MatchedPolicies) > 0 {
		_, _ = fmt.Fprintf(w, "Matched Policies:\t%s\n", strings.Join(res.MatchedPolicies, ", "))
	} else {
		_, _ = fmt.Fprintf(w, "Matched Policies:\tNone\n")
	}
	if len(res.CompileErrors) > 0 {
		_, _ = fmt.Fprintf(w, "Compile Errors:\t%s\n", strings.Join(res.CompileErrors, "; "))
	}
	_ = w.Flush()
}

// PrintDecisionTable prints simple check decision.
func PrintDecisionTable(res *pectl.DecisionResp) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Decision:\t%s\n", res.Decision)
	_, _ = fmt.Fprintf(w, "Matched Policy:\t%s\n", res.MatchedPolicyID)
	_, _ = fmt.Fprintf(w, "Evaluated At:\t%s\n", res.EvaluatedAt.Format(time.RFC3339))
	_ = w.Flush()
}

// PrintExplainTable prints explain decision details.
func PrintExplainTable(res *pectl.ExplainResp) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Decision:\t%s\n", res.Decision)
	_, _ = fmt.Fprintf(w, "Latency:\t%s\n", res.Latency)
	_, _ = fmt.Fprintf(w, "Final Reason:\t%s\n", res.FinalReason)
	_ = w.Flush()

	if len(res.MatchedPolicies) > 0 {
		fmt.Println("\nMatched Policies:")
		tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "POLICY ID\tEFFECT\tCLAUSE")
		for _, mp := range res.MatchedPolicies {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", mp.ID, mp.Effect, mp.PolicyText)
		}
		_ = tw.Flush()
	}
}

// PrintTenantsTable prints tenant details.
func PrintTenantsTable(tenants []pectl.Tenant) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TENANT ID\tNAME\tCREATED AT")
	for _, t := range tenants {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", t.ID, t.Name, t.CreatedAt.Format(time.RFC3339))
	}
	_ = w.Flush()
}

// PrintTenantStatusTable prints status properties.
func PrintTenantStatusTable(tenantID string, status *pectl.TenantStatus) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "PROPERTY\tVALUE\n")
	_, _ = fmt.Fprintf(w, "Tenant ID\t%s\n", tenantID)
	_, _ = fmt.Fprintf(w, "Active\t%t\n", status.Active)
	_, _ = fmt.Fprintf(w, "Evicted\t%t\n", status.Evicted)
	_, _ = fmt.Fprintf(w, "Memory Usage\t%s\n", status.MemoryUsage)
	_, _ = fmt.Fprintf(w, "Policy Count\t%d\n", status.PolicyCount)
	_, _ = fmt.Fprintf(w, "Last Activity\t%s\n", status.LastActivity.Format(time.RFC3339))
	_ = w.Flush()
}

// PrintMetricsTable prints metrics properties.
func PrintMetricsTable(metrics *pectl.MetricsResp) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "METRIC\tVALUE\n")
	_, _ = fmt.Fprintf(w, "Decision Rate\t%.2f/s\n", metrics.DecisionRate)
	_, _ = fmt.Fprintf(w, "QPS\t%.2f\n", metrics.QPS)
	_, _ = fmt.Fprintf(w, "Latency P50\t%.3f ms\n", metrics.LatencyP50)
	_, _ = fmt.Fprintf(w, "Latency P95\t%.3f ms\n", metrics.LatencyP95)
	_, _ = fmt.Fprintf(w, "Latency P99\t%.3f ms\n", metrics.LatencyP99)
	_, _ = fmt.Fprintf(w, "Memory Usage\t%d bytes\n", metrics.MemoryUsage)
	_, _ = fmt.Fprintf(w, "GC Statistics\t%s\n", metrics.GCStatistics)
	_ = w.Flush()
}

// PrintHealthTable prints health details.
func PrintHealthTable(health *pectl.HealthResp) {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Overall Health:\t%s\n\n", health.Status)
	_, _ = fmt.Fprintln(w, "COMPONENT\tSTATUS\tMESSAGE")
	for name, comp := range health.Components {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, comp.Status, comp.Message)
	}
	_ = w.Flush()
}


