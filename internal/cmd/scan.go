package cmd

import (
	"fmt"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the current project for services and configuration",
	Long: `Scan the current directory (and configured paths) for Paradox services,
Dockerfiles, docker-compose files, and other deployable artifacts.

This command will later feed into 'paradox deploy' and the Paradox Cloud
control plane.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.Info("scan started (stub)")
		fmt.Println("paradox scan — coming in the next iteration")
		fmt.Println()
		fmt.Println("Planned capabilities:")
		fmt.Println("  • Discover services from paradox.yaml")
		fmt.Println("  • Detect Dockerfiles / docker-compose.yml")
		fmt.Println("  • Detect language runtimes (Go, Node, Python, etc.)")
		fmt.Println("  • Report missing configuration")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
