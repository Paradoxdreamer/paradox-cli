package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	DefaultConfigName = "paradox"
	DefaultConfigType = "yaml"
	EnvPrefix         = "PARADOX"
)

// ValidEnvironments are the allowed values for environment.
var ValidEnvironments = []string{"development", "staging", "production", "test"}

// ValidLogLevels are the allowed values for log_level.
var ValidLogLevels = []string{"debug", "info", "warn", "error"}

// Config holds the global Paradox configuration.
// This is the single source of truth for project + platform settings.
// Future services (Auth, Queue, Deploy, …) should read from here
// or from nested sections that we add deliberately — never invent
// their own ad-hoc config loaders.
type Config struct {
	ProjectName string            `mapstructure:"project_name" yaml:"project_name"`
	Environment string            `mapstructure:"environment"  yaml:"environment"`
	LogLevel    string            `mapstructure:"log_level"    yaml:"log_level"`
	DataDir     string            `mapstructure:"data_dir"     yaml:"data_dir"`
	Services    []ServiceConfig   `mapstructure:"services"     yaml:"services"`
	// Reserved for future platform sections (auth, queue, storage, …).
	// Adding a field here is the *only* approved way to extend config.
	Extra map[string]interface{} `mapstructure:",remain"`
}

// ServiceConfig describes a service declared in paradox.yaml.
type ServiceConfig struct {
	Name string `mapstructure:"name" yaml:"name"`
	Path string `mapstructure:"path" yaml:"path"` // optional relative path
	Type string `mapstructure:"type" yaml:"type"` // optional: api, worker, web, …
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		ProjectName: "",
		Environment: "development",
		LogLevel:    "info",
		DataDir:     filepath.Join(home, ".paradox"),
		Services:    nil,
	}
}

// Load reads configuration from file, environment, and defaults.
// If configFile is non-empty, it is used as the explicit config path.
// After loading, Validate is *not* called automatically — callers
// that need strictness should call cfg.Validate() themselves
// (e.g. doctor, deploy, or a dedicated validate command).
func Load(configFile string) (*Config, error) {
	v := viper.New()

	v.SetConfigName(DefaultConfigName)
	v.SetConfigType(DefaultConfigType)

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("./.paradox")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "paradox"))
			v.AddConfigPath(filepath.Join(home, ".paradox"))
		}
	}

	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	// Map PARADOX_LOG_LEVEL → log_level, etc.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	cfg := Default()
	v.SetDefault("environment", cfg.Environment)
	v.SetDefault("log_level", cfg.LogLevel)
	v.SetDefault("data_dir", cfg.DataDir)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Missing file is fine — we still return defaults.
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Normalize
	cfg.Environment = strings.ToLower(strings.TrimSpace(cfg.Environment))
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	cfg.ProjectName = strings.TrimSpace(cfg.ProjectName)

	return cfg, nil
}

// EnsureDataDir creates the data directory if it does not exist.
func EnsureDataDir(cfg *Config) error {
	if cfg.DataDir == "" {
		return nil
	}
	return os.MkdirAll(cfg.DataDir, 0o755)
}

// ConfigFileUsed is kept for compatibility; prefer tracking the path at the call site.
func ConfigFileUsed() string {
	return ""
}
