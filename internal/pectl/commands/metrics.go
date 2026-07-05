package commands

import (
	"context"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"

	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Display engine telemetry metrics (decision rate, QPS, latency, memory)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := GetClient()
		var resp pectl.MetricsResp
		err := client.Request(context.Background(), "GET", "/api/v1/metrics", nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintMetricsTable(&resp)
		})
		HandleError(err)
	},
}

func init() {
	RootCmd.AddCommand(metricsCmd)
}
