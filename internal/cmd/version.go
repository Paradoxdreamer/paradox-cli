package cmd

import (
	"fmt"

	"github.com/paradox-cloud/paradox/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Paradox CLI",
	Long:  `Print the version number of Paradox CLI, including commit hash and build date when available.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("paradox %s\n", version.Version)
		if version.Commit != "none" {
			fmt.Printf("  commit: %s\n", version.Commit)
		}
		if version.BuildDate != "unknown" {
			fmt.Printf("  built:  %s\n", version.BuildDate)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
