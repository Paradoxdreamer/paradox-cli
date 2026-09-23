package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/scan"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan the current project for services and configuration",
	Long: `Scan the current directory (or the given path) for Paradox services,
Dockerfiles, docker-compose files, language runtimes, and other
deployable artifacts.

Results feed into 'paradox deploy' and the Paradox Cloud control plane.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		logging.Info("starting scan", "dir", dir)

		result, err := scan.Scan(dir)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		printScanResult(result)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

func printScanResult(r *scan.Result) {
	fmt.Println("Paradox Scan")
	fmt.Println("============")
	fmt.Printf("Root: %s\n", r.Root)
	fmt.Println()

	// Project config
	fmt.Println("Project")
	fmt.Println("-------")
	if r.HasParadoxYAML {
		rel := relPath(r.Root, r.ParadoxYAML)
		name := r.ProjectName
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  ✓ paradox.yaml     %s\n", rel)
		fmt.Printf("    project_name:    %s\n", name)
		if len(r.Services) > 0 {
			fmt.Printf("    services:        %s\n", strings.Join(r.Services, ", "))
		} else {
			fmt.Printf("    services:        (none declared)\n")
		}
	} else {
		fmt.Println("  ✗ paradox.yaml     not found")
	}
	fmt.Println()

	// Docker
	fmt.Println("Containers")
	fmt.Println("----------")
	if len(r.Dockerfiles) == 0 && len(r.ComposeFiles) == 0 {
		fmt.Println("  (none found)")
	} else {
		for _, f := range r.Dockerfiles {
			fmt.Printf("  ✓ Dockerfile       %s\n", relPath(r.Root, f))
		}
		for _, f := range r.ComposeFiles {
			fmt.Printf("  ✓ Compose          %s\n", relPath(r.Root, f))
		}
	}
	fmt.Println()

	// Runtimes
	fmt.Println("Runtimes")
	fmt.Println("--------")
	if len(r.Runtimes) == 0 {
		fmt.Println("  (none detected)")
	} else {
		for _, rt := range r.Runtimes {
			fmt.Printf("  ✓ %-14s  %s\n", rt.Name, relPath(r.Root, rt.Marker))
		}
	}
	fmt.Println()

	// Missing / suggestions
	if len(r.Missing) > 0 {
		fmt.Println("Suggestions")
		fmt.Println("-----------")
		for _, m := range r.Missing {
			fmt.Printf("  • %s\n", m)
		}
		fmt.Println()
	}

	// Summary line
	total := 0
	if r.HasParadoxYAML {
		total++
	}
	total += len(r.Dockerfiles) + len(r.ComposeFiles) + len(r.Runtimes)
	logging.Info("scan complete",
		"dockerfiles", len(r.Dockerfiles),
		"compose", len(r.ComposeFiles),
		"runtimes", len(r.Runtimes),
		"has_config", r.HasParadoxYAML,
	)
	fmt.Printf("Found %d artifact(s).\n", total)
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
