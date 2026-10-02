package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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

var rootCmd = &cobra.Command{
	Use:   "paradox",
	Short: "Paradox — one CLI for your whole ecosystem",
	Long: `Paradox is the single entry point for the Paradox Cloud platform.

Use it to initialize projects, manage services, deploy, diagnose issues,
and interact with Auth, Queue, Storage, Feature Flags, and more.

  prx init / paradox init      Initialize a project
  prx doctor / paradox doctor  Check environment
  prx cloud up                 Start local cloud
`,
	Version: version.Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		level, err := logging.ParseLevel(logLevel)
		if err != nil {
			return err
		}
		format, err := logging.ParseFormat(logFormat)
		if err != nil {
			return err
		}
		logging.Configure(logging.Options{Level: level, Format: format})

		cfg, err = config.Load(cfgFile)
		if err != nil {
			return err
		}

		if logLevel == "" && cfg.LogLevel != "" {
			if l, err := logging.ParseLevel(cfg.LogLevel); err == nil {
				logging.Configure(logging.Options{Level: l, Format: format})
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

// Execute runs the CLI. Binary may be named paradox or prx.
func Execute() error {
	name := filepath.Base(os.Args[0])
	if name == "prx" || name == "paradox" {
		rootCmd.Use = name
	}
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.paradox/paradox.yaml or ./.paradox/paradox.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug|info|warn|error (default: info, or from config)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format: text|json")
	rootCmd.SetVersionTemplate(fmt.Sprintf("paradox %s (commit: %s, built: %s)\n", version.Version, version.Commit, version.BuildDate))
	rootCmd.SilenceUsage = true
}

func GetConfig() *config.Config {
	return cfg
}
