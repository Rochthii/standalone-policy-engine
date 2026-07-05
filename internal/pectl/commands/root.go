package commands

import (
	"fmt"
	"os"
	"time"

	"standalone-policy-engine/internal/pectl"
	"standalone-policy-engine/internal/pectl/printer"

	"github.com/spf13/cobra"
)

var (
	CfgFile      string
	ServerFlag   string
	TokenFlag    string
	OutputFlag   string
	TimeoutFlag  string
	ActiveConfig *pectl.Config
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "pectl",
	Short: "pectl is the CLI control tool for the Standalone Policy Engine",
	Long: `A production-ready CLI for policy lifecycle management, simulations, 
decision queries, telemetry, health checks, and automation workflows.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := pectl.LoadConfig(CfgFile)
		if err != nil {
			return err
		}

		// Override config values with flags if explicitly provided
		if cmd.Flags().Changed("server") {
			cfg.Server = ServerFlag
		}
		if cmd.Flags().Changed("token") {
			cfg.Token = TokenFlag
		}
		if cmd.Flags().Changed("output") {
			cfg.Output = OutputFlag
		}
		if cmd.Flags().Changed("timeout") {
			d, err := time.ParseDuration(TimeoutFlag)
			if err != nil {
				return fmt.Errorf("invalid timeout value: %w", err)
			}
			cfg.Timeout = d
		}

		// Validate output flag
		if cfg.Output != "table" && cfg.Output != "json" && cfg.Output != "yaml" {
			return fmt.Errorf("invalid output format '%s', must be one of: table, json, yaml", cfg.Output)
		}

		ActiveConfig = cfg
		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&CfgFile, "config", "", "config file (default is $HOME/.pectl/config.yaml)")
	RootCmd.PersistentFlags().StringVar(&ServerFlag, "server", "", "REST Control Plane server URL (e.g. http://localhost:8080)")
	RootCmd.PersistentFlags().StringVar(&TokenFlag, "token", "", "JWT authentication token")
	RootCmd.PersistentFlags().StringVar(&OutputFlag, "output", "", "output format (table, json, yaml)")
	RootCmd.PersistentFlags().StringVar(&TimeoutFlag, "timeout", "", "request timeout (e.g. 5s, 10s)")
}

// GetClient returns an initialized pectl.Client using the active config.
func GetClient() *pectl.Client {
	return pectl.NewClient(ActiveConfig.Server, ActiveConfig.Token, ActiveConfig.Timeout)
}

// PrintResult prints the data using the selected output mode.
func PrintResult(data interface{}, tableFunc func()) error {
	switch ActiveConfig.Output {
	case "json":
		return printer.PrintJSON(data)
	case "yaml":
		return printer.PrintYAML(data)
	default:
		tableFunc()
		return nil
	}
}

// HandleError handles command errors with correct exit codes.
func HandleError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
