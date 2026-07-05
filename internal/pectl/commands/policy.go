package commands

import (
	"context"
	"fmt"
	"os"
	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"

	"github.com/spf13/cobra"
)

var (
	policyEffect string
	policyFile   string
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage policies on the control plane",
}

var createPolicyCmd = &cobra.Command{
	Use:   "create <tenant_id>",
	Short: "Create a new draft policy",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		if policyFile == "" {
			HandleError(fmt.Errorf("--file flag is required"))
		}

		data, err := os.ReadFile(policyFile)
		if err != nil {
			HandleError(fmt.Errorf("failed to read policy file: %w", err))
		}

		client := GetClient()
		req := pectl.CreatePolicyReq{
			Effect:     policyEffect,
			PolicyText: string(data),
		}

		var resp pectl.CreatePolicyResp
		err = client.Request(context.Background(), "POST", fmt.Sprintf("/api/v1/tenants/%s/policies", tenantID), req, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			fmt.Printf("Policy created successfully. ID: %s (Status: %s)\n", resp.PolicyID, resp.Status)
		})
		HandleError(err)
	},
}

var updatePolicyCmd = &cobra.Command{
	Use:   "update <tenant_id> <policy_id>",
	Short: "Update an existing draft policy",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		policyID := args[1]

		if policyFile == "" {
			HandleError(fmt.Errorf("--file flag is required"))
		}

		data, err := os.ReadFile(policyFile)
		if err != nil {
			HandleError(fmt.Errorf("failed to read policy file: %w", err))
		}

		client := GetClient()
		req := pectl.UpdatePolicyReq{
			PolicyText: string(data),
		}

		var resp pectl.UpdatePolicyResp
		err = client.Request(context.Background(), "PUT", fmt.Sprintf("/api/v1/tenants/%s/policies/%s", tenantID, policyID), req, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			fmt.Printf("Policy %s updated successfully (Status: %s)\n", policyID, resp.Status)
		})
		HandleError(err)
	},
}

var publishPolicyCmd = &cobra.Command{
	Use:   "publish <tenant_id> <policy_id>",
	Short: "Publish a draft policy to active state",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		policyID := args[1]

		client := GetClient()
		var resp pectl.PublishPolicyResp
		err := client.Request(context.Background(), "POST", fmt.Sprintf("/api/v1/tenants/%s/policies/%s/publish", tenantID, policyID), nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			fmt.Printf("Policy %s published successfully.\nVersion: %d\nPublished At: %v\nSync Cluster Event: %t\n",
				resp.PolicyID, resp.PublishedVersion, resp.PublishedAt, resp.SyncEventSent)
		})
		HandleError(err)
	},
}

var deletePolicyCmd = &cobra.Command{
	Use:   "delete <tenant_id> <policy_id>",
	Short: "Delete a policy",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		policyID := args[1]

		client := GetClient()
		var resp map[string]string
		err := client.Request(context.Background(), "DELETE", fmt.Sprintf("/api/v1/tenants/%s/policies/%s", tenantID, policyID), nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			if msg, ok := resp["message"]; ok {
				fmt.Println(msg)
			} else {
				fmt.Printf("Policy %s deleted successfully.\n", policyID)
			}
		})
		HandleError(err)
	},
}

var listPoliciesCmd = &cobra.Command{
	Use:   "list <tenant_id>",
	Short: "List all policies for a tenant",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]

		client := GetClient()
		var resp []pectl.Policy
		err := client.Request(context.Background(), "GET", fmt.Sprintf("/api/v1/tenants/%s/policies", tenantID), nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintPoliciesTable(resp)
		})
		HandleError(err)
	},
}

var getPolicyCmd = &cobra.Command{
	Use:   "get <tenant_id> <policy_id>",
	Short: "Get details of a specific policy",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tenantID := args[0]
		policyID := args[1]

		client := GetClient()
		var resp pectl.Policy
		err := client.Request(context.Background(), "GET", fmt.Sprintf("/api/v1/tenants/%s/policies/%s", tenantID, policyID), nil, &resp)
		HandleError(err)

		err = PrintResult(resp, func() {
			printer.PrintPolicyDetailTable(&resp)
		})
		HandleError(err)
	},
}

func init() {
	createPolicyCmd.Flags().StringVar(&policyEffect, "effect", "permit", "Effect of the policy (permit or forbid)")
	createPolicyCmd.Flags().StringVar(&policyFile, "file", "", "Path to the cedar policy file")
	_ = createPolicyCmd.MarkFlagRequired("file")

	updatePolicyCmd.Flags().StringVar(&policyFile, "file", "", "Path to the cedar policy file")
	_ = updatePolicyCmd.MarkFlagRequired("file")

	policyCmd.AddCommand(createPolicyCmd)
	policyCmd.AddCommand(updatePolicyCmd)
	policyCmd.AddCommand(publishPolicyCmd)
	policyCmd.AddCommand(deletePolicyCmd)
	policyCmd.AddCommand(listPoliciesCmd)
	policyCmd.AddCommand(getPolicyCmd)

	RootCmd.AddCommand(policyCmd)
}
