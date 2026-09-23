package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultConfigName = "paradox"
	DefaultConfigType = "yaml"
	EnvPrefix         = "PARADOX"
)

// Config holds the global Paradox configuration.
type Config struct {
	ProjectName string `mapstructure:"project_name"`
	Environment string `mapstructure:"environment"`
	LogLevel    string `mapstructure:"log_level"`
	DataDir     string `mapstructure:"data_dir"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		ProjectName: "",
		Environment: "development",
		LogLevel:    "info",
		DataDir:     filepath.Join(home, ".paradox"),
	}
}

// Load reads configuration from file, environment, and defaults.
// If configFile is non-empty, it is used as the explicit config path.
func Load(configFile string) (*Config, error) {
	v := viper.New()

	v.SetConfigName(DefaultConfigName)
	v.SetConfigType(DefaultConfigType)

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		// Search paths (project local first, then user config)
		v.AddConfigPath(".")
		v.AddConfigPath("./.paradox")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "paradox"))
			v.AddConfigPath(filepath.Join(home, ".paradox"))
		}
	}

	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()

	// Defaults
	cfg := Default()
	v.SetDefault("environment", cfg.Environment)
	v.SetDefault("log_level", cfg.LogLevel)
	v.SetDefault("data_dir", cfg.DataDir)

	// Read config file if present (ignore missing)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// ConfigFileUsed is no longer global; callers should track the path if needed.
func ConfigFileUsed() string {
	return ""
}

// EnsureDataDir creates the data directory if it does not exist.
func EnsureDataDir(cfg *Config) error {
	if cfg.DataDir == "" {
		return nil
	}
	return os.MkdirAll(cfg.DataDir, 0o755)
}
