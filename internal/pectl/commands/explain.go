package commands

import (
	"context"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"
	"time"

	"github.com/spf13/cobra"
)

var (
	explainSubject  string
	explainAction   string
	explainResource string
)

var explainCmd = &cobra.Command{
	Use:   "explain <tenant_id>",
	Short: "Explain the decision for an access request, showing matched policies and trace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		req := pectl.DecisionReq{
			TenantID: tenantID,
			Subject:  explainSubject,
			Action:   explainAction,
			Resource: explainResource,
		}

		client := GetClient()
		var resp pectl.ExplainResp
		start := time.Now()
		err := client.Request(context.Background(), "POST", "/api/v1/decisions/explain", req, &resp)
		elapsed := time.Since(start)
		HandleError(err)

		// Inject latency info
		resp.Latency = elapsed.Round(time.Microsecond).String()

		err = PrintResult(resp, func() {
			printer.PrintExplainTable(&resp)
		})
		HandleError(err)
	},
}

func init() {
	explainCmd.Flags().StringVar(&explainSubject, "subject", "", "Subject of the request (e.g. user:alice)")
	explainCmd.Flags().StringVar(&explainAction, "action", "", "Action to check (e.g. read)")
	explainCmd.Flags().StringVar(&explainResource, "resource", "", "Resource to check (e.g. invoice-123)")

	_ = explainCmd.MarkFlagRequired("subject")
	_ = explainCmd.MarkFlagRequired("action")
	_ = explainCmd.MarkFlagRequired("resource")

	RootCmd.AddCommand(explainCmd)
}
