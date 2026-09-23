package cmd

import (
	"fmt"

	"github.com/paradox-cloud/paradox/internal/config"
	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and validate Paradox configuration",
	Long: `Work with paradox.yaml and related configuration.

  paradox config validate   Validate the current config against the schema
  paradox config show       Print the effective configuration
`,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate paradox.yaml against the platform schema",
	Long: `Load the current configuration and run the full schema validation.

This is the single source of truth for "is this config legal?".
Auth, Queue, Deploy and other services should rely on this rather
than inventing their own checks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("config not loaded")
		}

		logging.Debug("validating config")
		errs := cfg.Validate()
		if errs.Empty() {
			fmt.Println("✓ Configuration is valid")
			logging.Info("config validation passed")
			return nil
		}

		fmt.Println("✗ Configuration is invalid")
		fmt.Println()
		for _, e := range errs {
			fmt.Printf("  • %s\n", e.Error())
		}
		logging.Warn("config validation failed", "errors", len(errs))
		return fmt.Errorf("config validation failed (%d error(s))", len(errs))
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg == nil {
			return fmt.Errorf("config not loaded")
		}

		fmt.Println("Effective configuration")
		fmt.Println("======================")
		fmt.Printf("  project_name:  %s\n", emptyAs(cfg.ProjectName, "(not set)"))
		fmt.Printf("  environment:   %s\n", cfg.Environment)
		fmt.Printf("  log_level:     %s\n", cfg.LogLevel)
		fmt.Printf("  data_dir:      %s\n", cfg.DataDir)
		fmt.Println()
		if len(cfg.Services) == 0 {
			fmt.Println("  services:      (none)")
		} else {
			fmt.Println("  services:")
			for _, s := range cfg.Services {
				extra := ""
				if s.Path != "" {
					extra += " path=" + s.Path
				}
				if s.Type != "" {
					extra += " type=" + s.Type
				}
				fmt.Printf("    - %s%s\n", s.Name, extra)
			}
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

func emptyAs(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// Keep the config package referenced.
var _ = config.Default
