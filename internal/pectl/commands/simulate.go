package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"

	"github.com/spf13/cobra"
)

var (
	simSubject       string
	simAction        string
	simResource      string
	simContextFile   string
	simDraftFile     string
	simIncludeActive bool
)

var simulateCmd = &cobra.Command{
	Use:   "simulate <tenant_id>",
	Short: "Run a policy evaluation simulation",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		var req pectl.SimulateReq
		req.Subject = simSubject
		req.Action = simAction
		req.Resource = simResource
		req.IncludeActive = simIncludeActive

		// Load context JSON if provided
		if simContextFile != "" {
			data, err := os.ReadFile(simContextFile)
			if err != nil {
				HandleError(fmt.Errorf("failed to read context file: %w", err))
			}
			var ctxMap map[string]string
			if err := json.Unmarshal(data, &ctxMap); err != nil {
				HandleError(fmt.Errorf("invalid context JSON: %w", err))
			}
			req.Context = ctxMap
		}

		// Load draft policy if provided
		if simDraftFile != "" {
			data, err := os.ReadFile(simDraftFile)
			if err != nil {
				HandleError(fmt.Errorf("failed to read draft policy file: %w", err))
			}
			req.Policies = []pectl.SimulatePolicy{
				{
					ID:         "draft-policy",
					PolicyText: string(data),
				},
			}
		}

		client := GetClient()
		var resp pectl.SimulateResp
		err := client.Request(context.Background(), "POST", fmt.Sprintf("/api/v1/tenants/%s/simulate", tenantID), req, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintSimulationTable(&resp)
		})
		HandleError(err)
	},
}

func init() {
	simulateCmd.Flags().StringVar(&simSubject, "subject", "", "Subject of the request")
	simulateCmd.Flags().StringVar(&simAction, "action", "", "Action of the request")
	simulateCmd.Flags().StringVar(&simResource, "resource", "", "Resource of the request")
	simulateCmd.Flags().StringVar(&simContextFile, "context-file", "", "Path to the JSON file containing dynamic context attributes")
	simulateCmd.Flags().StringVar(&simDraftFile, "draft-file", "", "Path to a draft cedar policy file to evaluate")
	simulateCmd.Flags().BoolVar(&simIncludeActive, "include-active", false, "Include active policies in evaluation along with the draft")

	_ = simulateCmd.MarkFlagRequired("subject")
	_ = simulateCmd.MarkFlagRequired("action")
	_ = simulateCmd.MarkFlagRequired("resource")

	RootCmd.AddCommand(simulateCmd)
}
