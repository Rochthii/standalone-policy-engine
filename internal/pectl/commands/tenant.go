package commands

import (
	"context"
	"fmt"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"

	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage and inspect tenants",
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered tenants",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := GetClient()
		var resp []pectl.Tenant
		err := client.Request(context.Background(), "GET", "/api/v1/tenants", nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintTenantsTable(resp)
		})
		HandleError(err)
	},
}

var tenantGetCmd = &cobra.Command{
	Use:   "get <tenant_id>",
	Short: "Get details of a specific tenant",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		client := GetClient()
		var resp pectl.Tenant
		err := client.Request(context.Background(), "GET", fmt.Sprintf("/api/v1/tenants/%s", tenantID), nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintTenantsTable([]pectl.Tenant{resp})
		})
		HandleError(err)
	},
}

var tenantStatusCmd = &cobra.Command{
	Use:   "status <tenant_id>",
	Short: "Show operational status of a tenant (active, evicted, memory, policy count, last activity)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		client := GetClient()
		var resp pectl.TenantStatus
		err := client.Request(context.Background(), "GET", fmt.Sprintf("/api/v1/tenants/%s/status", tenantID), nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintTenantStatusTable(tenantID, &resp)
		})
		HandleError(err)
	},
}

func init() {
	tenantCmd.AddCommand(tenantListCmd)
	tenantCmd.AddCommand(tenantGetCmd)
	tenantCmd.AddCommand(tenantStatusCmd)
	RootCmd.AddCommand(tenantCmd)
}
