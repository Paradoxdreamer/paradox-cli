package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/paradox-cloud/paradox/internal/config"
	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/version"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check your Paradox environment and configuration",
	Long: `Run a series of diagnostic checks on your local environment,
configuration, and dependencies. Useful for troubleshooting setup issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logging.Debug("running doctor checks")

		fmt.Println("Paradox Doctor")
		fmt.Println("==============")
		fmt.Println()

		checks := []struct {
			name string
			fn   func() (ok bool, detail string)
		}{
			{"CLI version", checkVersion},
			{"Go runtime", checkGoRuntime},
			{"Configuration", checkConfig},
			{"Data directory", checkDataDir},
			{"Project config", checkProjectConfig},
			{"Git available", checkGit},
			{"Docker available", checkDocker},
		}

		allOK := true
		for _, c := range checks {
			ok, detail := c.fn()
			status := "✓"
			if !ok {
				status = "✗"
				allOK = false
				logging.Warn("doctor check failed", "check", c.name, "detail", detail)
			} else {
				logging.Debug("doctor check passed", "check", c.name, "detail", detail)
			}
			fmt.Printf("%s  %-20s %s\n", status, c.name, detail)
		}

		fmt.Println()
		if allOK {
			logging.Info("all doctor checks passed")
			fmt.Println("All checks passed. You're ready to go.")
		} else {
			logging.Warn("some doctor checks failed")
			fmt.Println("Some checks failed. Fix the issues above and re-run `paradox doctor`.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func checkVersion() (bool, string) {
	return true, version.Version
}

func checkGoRuntime() (bool, string) {
	return true, fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func checkConfig() (bool, string) {
	cfg := GetConfig()
	if cfg == nil {
		return false, "config not loaded"
	}
	return true, fmt.Sprintf("environment=%s log_level=%s", cfg.Environment, cfg.LogLevel)
}

func checkDataDir() (bool, string) {
	cfg := GetConfig()
	if cfg == nil || cfg.DataDir == "" {
		return false, "data_dir not set"
	}
	info, err := os.Stat(cfg.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Sprintf("%s (does not exist)", cfg.DataDir)
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, fmt.Sprintf("%s is not a directory", cfg.DataDir)
	}
	return true, cfg.DataDir
}

func checkProjectConfig() (bool, string) {
	// Look for paradox.yaml in current dir or .paradox/
	candidates := []string{
		"paradox.yaml",
		"paradox.yml",
		filepath.Join(".paradox", "paradox.yaml"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true, c
		}
	}
	return false, "no paradox.yaml found (run `paradox init`)"
}

func checkGit() (bool, string) {
	path, err := exec.LookPath("git")
	if err != nil {
		return false, "git not found in PATH"
	}
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return true, path
	}
	return true, strings.TrimSpace(string(out))
}

func checkDocker() (bool, string) {
	path, err := exec.LookPath("docker")
	if err != nil {
		return false, "docker not found (optional for local services)"
	}
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		// Docker binary exists but daemon may not be running
		return true, path + " (daemon not reachable)"
	}
	return true, "docker " + strings.TrimSpace(string(out))
}

// Ensure config package is used
var _ = config.Default
