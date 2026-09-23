package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

// Package auth is the minimal Paradox Auth service.
//
// Scope for v0 (this file):
//   - Register with the CLI registry
//   - Local identity store under data_dir/auth
//   - Basic commands: status, init, whoami
//   - Doctor check
//
// Not yet: JWT issuance, OAuth, RBAC, refresh tokens, HTTP API.
// Those come once the shape is proven.

const serviceName = "auth"

func init() {
	registry.Register(&registry.Service{
		Name:          serviceName,
		Description:   "Identity & authentication (JWT, sessions, RBAC)",
		ConfigSection: "auth",
		Commands: []*cobra.Command{
			authCmd,
		},
		DoctorChecks: []registry.DoctorCheck{
			{Name: "Auth store", Fn: checkAuthStore},
		},
	})
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Paradox Auth — identity and authentication",
	Long: `Paradox Auth provides identity for the platform.

Current (v0):
  paradox auth status    Show auth service status
  paradox auth init      Initialize local auth store
  paradox auth whoami    Show current identity (local)

Coming: JWT + refresh, OAuth, RBAC, sessions, rate limiting.`,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Auth service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storePath()
		if err != nil {
			return err
		}
		initialized := dirExists(store)

		fmt.Println("Paradox Auth")
		fmt.Println("============")
		fmt.Printf("  Status:       %s\n", map[bool]string{true: "initialized", false: "not initialized"}[initialized])
		fmt.Printf("  Store:        %s\n", store)
		if initialized {
			fmt.Printf("  Users file:   %s\n", filepath.Join(store, "users.json"))
		} else {
			fmt.Println()
			fmt.Println("  Run `paradox auth init` to create the local store.")
		}
		logging.Info("auth status", "initialized", initialized, "store", store)
		return nil
	},
}

var authInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the local Auth store",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storePath()
		if err != nil {
			return err
		}
		if dirExists(store) {
			fmt.Printf("Auth store already exists at %s\n", store)
			return nil
		}
		if err := os.MkdirAll(store, 0o700); err != nil {
			return fmt.Errorf("create auth store: %w", err)
		}
		// Minimal users file
		usersPath := filepath.Join(store, "users.json")
		content := fmt.Sprintf("{\n  \"created_at\": %q,\n  \"users\": []\n}\n", time.Now().UTC().Format(time.RFC3339))
		if err := os.WriteFile(usersPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write users.json: %w", err)
		}
		fmt.Printf("✓ Auth store initialized at %s\n", store)
		logging.Info("auth store initialized", "path", store)
		return nil
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current identity (local stub)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// v0: no real sessions yet
		fmt.Println("Not signed in (local auth has no active session yet).")
		fmt.Println()
		fmt.Println("Next: JWT issuance + `paradox auth login` will land here.")
		return nil
	},
}

func init() {
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authInitCmd)
	authCmd.AddCommand(authWhoamiCmd)
}

func storePath() (string, error) {
	// Prefer PARADOX_DATA_DIR / config data_dir; fall back to ~/.paradox/auth
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "auth"), nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func checkAuthStore() (ok bool, detail string) {
	store, err := storePath()
	if err != nil {
		return false, err.Error()
	}
	if !dirExists(store) {
		return false, "not initialized (run `paradox auth init`)"
	}
	return true, store
}
