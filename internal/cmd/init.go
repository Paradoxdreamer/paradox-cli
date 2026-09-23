package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/spf13/cobra"
)

var (
	initProjectName string
	initForce       bool
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new Paradox project",
	Long: `Initialize a new Paradox project in the current directory (or the given directory).

Creates:
  - .paradox/                  project-local config & state
  - paradox.yaml               project configuration
  - .gitignore                 sensible defaults
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		logging.Debug("initializing project", "dir", absDir)

		if err := os.MkdirAll(absDir, 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		paradoxDir := filepath.Join(absDir, ".paradox")
		if _, err := os.Stat(paradoxDir); err == nil && !initForce {
			return fmt.Errorf("project already initialized at %s (use --force to overwrite)", absDir)
		}

		if err := os.MkdirAll(paradoxDir, 0o755); err != nil {
			return fmt.Errorf("create .paradox: %w", err)
		}

		projectName := initProjectName
		if projectName == "" {
			projectName = filepath.Base(absDir)
		}

		configPath := filepath.Join(absDir, "paradox.yaml")
		configContent := fmt.Sprintf(`# Paradox project configuration
project_name: %s
environment: development
log_level: info

# Services that will be managed by this project
services: []

# Future: auth, queue, storage, feature_flags, etc.
`, projectName)

		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			return fmt.Errorf("write paradox.yaml: %w", err)
		}
		logging.Debug("wrote config", "path", configPath)

		gitignorePath := filepath.Join(absDir, ".gitignore")
		gitignoreContent := `# Paradox
.paradox/
*.log
.env
.env.*
!.env.example

# Build / local
/bin/
/dist/
/tmp/
`

		// Only write .gitignore if it doesn't exist, or force
		if _, err := os.Stat(gitignorePath); os.IsNotExist(err) || initForce {
			if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
				return fmt.Errorf("write .gitignore: %w", err)
			}
			logging.Debug("wrote .gitignore", "path", gitignorePath)
		}

		logging.Info("project initialized", "name", projectName, "path", absDir)

		fmt.Printf("✓ Initialized Paradox project %q in %s\n", projectName, absDir)
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  paradox doctor     # verify your environment")
		fmt.Println("  paradox scan       # discover services (coming soon)")
		fmt.Println("  paradox deploy     # deploy (coming soon)")
		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initProjectName, "name", "n", "", "project name (default: directory name)")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing configuration")
	rootCmd.AddCommand(initCmd)
}

// Helper to check if a string is empty or whitespace
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
