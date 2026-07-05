package commands

import (
	"context"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health status of all control plane components",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := GetClient()
		var resp pectl.HealthResp
		err := client.Request(context.Background(), "GET", "/api/v1/health", nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintHealthTable(&resp)
		})
		HandleError(err)
	},
}

func init() {
	RootCmd.AddCommand(healthCmd)
}
