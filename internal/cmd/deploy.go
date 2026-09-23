package cmd

import (
	"fmt"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy the current project",
	Long: `Build and deploy the current Paradox project.

This is the entry point for the Paradox Deploy service:
  git push → build → deploy → health check → rollback

In this early version the command is a stub. Full implementation
will live in the Paradox Deploy component.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.Info("deploy started (stub)")
		fmt.Println("paradox deploy — coming soon")
		fmt.Println()
		fmt.Println("This command will eventually:")
		fmt.Println("  1. Read paradox.yaml and discovered services")
		fmt.Println("  2. Build containers / binaries")
		fmt.Println("  3. Push to registry (or local)")
		fmt.Println("  4. Deploy with health checks")
		fmt.Println("  5. Support rollback")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
