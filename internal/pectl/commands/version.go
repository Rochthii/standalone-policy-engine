package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information injected at build time via ldflags.
var (
	Version   = "dev"
	GitCommit = "none"
	BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print CLI version, git commit, and build time",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("pectl version: %s\n", Version)
		fmt.Printf("Git Commit:    %s\n", GitCommit)
		fmt.Printf("Build Time:    %s\n", BuildTime)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
