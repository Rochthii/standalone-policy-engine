package main

import (
	"fmt"
	"os"
	"standalone-policy-engine/internal/pectl/commands"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
