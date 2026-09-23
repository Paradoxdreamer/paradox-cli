package cmd

import (
	"fmt"
	"os"

	"github.com/paradox-cloud/paradox/internal/config"
	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/version"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	cfg       *config.Config
	logLevel  string
	logFormat string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "paradox",
	Short: "Paradox — one CLI for your whole ecosystem",
	Long: `Paradox is the single entry point for the Paradox Cloud platform.

Use it to initialize projects, manage services, deploy, diagnose issues,
and interact with Auth, Queue, Storage, Feature Flags, and more.

  paradox init      Initialize a new Paradox project
  paradox doctor    Check your environment and configuration
  paradox version   Show version information
`,
	Version: version.Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 1. Configure logging first so everything else can log
		level, err := logging.ParseLevel(logLevel)
		if err != nil {
			return err
		}
		format, err := logging.ParseFormat(logFormat)
		if err != nil {
			return err
		}
		logging.Configure(logging.Options{
			Level:  level,
			Format: format,
		})

		// 2. Load config
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return err
		}

		// Allow config to override log level if flag was left at default
		if logLevel == "" && cfg.LogLevel != "" {
			if l, err := logging.ParseLevel(cfg.LogLevel); err == nil {
				logging.Configure(logging.Options{
					Level:  l,
					Format: format,
				})
			}
		}

		if err := config.EnsureDataDir(cfg); err != nil {
			return err
		}

		logging.Debug("config loaded",
			"environment", cfg.Environment,
			"log_level", cfg.LogLevel,
			"data_dir", cfg.DataDir,
		)
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.paradox/paradox.yaml or ./.paradox/paradox.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug|info|warn|error (default: info, or from config)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format: text|json")

	rootCmd.SetVersionTemplate(fmt.Sprintf("paradox %s (commit: %s, built: %s)\n", version.Version, version.Commit, version.BuildDate))

	// Silence usage on error for cleaner output
	rootCmd.SilenceUsage = true
}

// GetConfig returns the loaded configuration (available after PersistentPreRun).
func GetConfig() *config.Config {
	return cfg
}

// exitWithError prints an error and exits with code 1.
// Prefer returning errors from RunE so cobra can handle them.
func exitWithError(err error) {
	logging.Error("fatal", "error", err)
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
