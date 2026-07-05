package commands

import (
	"context"
	"fmt"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"
	"time"

	"github.com/spf13/cobra"
)

var (
	checkSubject  string
	checkAction   string
	checkResource string
)

var checkCmd = &cobra.Command{
	Use:   "check <tenant_id>",
	Short: "Check if a subject has permission to perform an action on a resource",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		req := pectl.DecisionReq{
			TenantID: tenantID,
			Subject:  checkSubject,
			Action:   checkAction,
			Resource: checkResource,
		}

		client := GetClient()
		var resp pectl.DecisionResp
		start := time.Now()
		err := client.Request(context.Background(), "POST", "/api/v1/decisions", req, &resp)
		elapsed := time.Since(start)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintDecisionTable(&resp)
			fmt.Printf("Latency: %s\n", elapsed.Round(time.Microsecond))
		})
		HandleError(err)
	},
}

func init() {
	checkCmd.Flags().StringVar(&checkSubject, "subject", "", "Subject of the request (e.g. user:alice)")
	checkCmd.Flags().StringVar(&checkAction, "action", "", "Action to check (e.g. read)")
	checkCmd.Flags().StringVar(&checkResource, "resource", "", "Resource to check (e.g. invoice-123)")

	_ = checkCmd.MarkFlagRequired("subject")
	_ = checkCmd.MarkFlagRequired("action")
	_ = checkCmd.MarkFlagRequired("resource")

	RootCmd.AddCommand(checkCmd)
}
