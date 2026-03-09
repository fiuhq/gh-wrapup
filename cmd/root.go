package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wrapup",
	Short: "Wrap up your work into an issue + PR",
	Long: `gh wrapup — You cooked. Now wrap it up.

Atomically creates a GitHub issue + PR as a single unit of work.
Creates the issue for human context and the PR that closes it, in one command.`,
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(upsertCmd)
}
