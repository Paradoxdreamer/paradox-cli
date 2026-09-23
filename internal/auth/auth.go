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

// Package auth is Paradox Auth.
//
// v0.1:
//   - Local user store (bcrypt)
//   - JWT issuance + validation (HS256)
//   - Commands: init, status, register, login, logout, whoami, token
//
// Still local-only (no HTTP API yet). JWT secret in store or PARADOX_JWT_SECRET.

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

	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authInitCmd)
	authCmd.AddCommand(authRegisterCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authWhoamiCmd)
	authCmd.AddCommand(authTokenCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Paradox Auth — identity and authentication",
	Long: `Paradox Auth provides identity for the platform.

  paradox auth init        Initialize local auth store
  paradox auth status      Show auth service status
  paradox auth register    Create a local user
  paradox auth login       Sign in and receive a JWT
  paradox auth logout      Clear local session
  paradox auth whoami      Show identity from session token
  paradox auth token       Print the current session JWT
`,
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
		users, _ := ListUsers()

		fmt.Println("Paradox Auth")
		fmt.Println("============")
		fmt.Printf("  Status:       %s\n", map[bool]string{true: "initialized", false: "not initialized"}[initialized])
		fmt.Printf("  Store:        %s\n", store)
		if initialized {
			fmt.Printf("  Users:        %d\n", len(users))
			fmt.Printf("  JWT key:      %s\n", filepath.Join(store, "jwt.key"))
		} else {
			fmt.Println()
			fmt.Println("  Run `paradox auth init` to create the local store.")
		}
		logging.Info("auth status", "initialized", initialized, "users", len(users))
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
		usersPath := filepath.Join(store, "users.json")
		content := fmt.Sprintf("{\n  \"created_at\": %q,\n  \"users\": []\n}\n", time.Now().UTC().Format(time.RFC3339))
		if err := os.WriteFile(usersPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write users.json: %w", err)
		}
		if _, err := signingKey(); err != nil {
			return err
		}
		fmt.Printf("✓ Auth store initialized at %s\n", store)
		logging.Info("auth store initialized", "path", store)
		return nil
	},
}

var (
	registerEmail    string
	registerPassword string
	registerRole     string
)

var authRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Create a local user",
	RunE: func(cmd *cobra.Command, args []string) error {
		if registerEmail == "" || registerPassword == "" {
			return fmt.Errorf("--email and --password are required")
		}
		if len(registerPassword) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}
		u, err := CreateUser(registerEmail, registerPassword, registerRole)
		if err != nil {
			return err
		}
		fmt.Printf("✓ User created\n")
		fmt.Printf("  id:    %s\n", u.ID)
		fmt.Printf("  email: %s\n", u.Email)
		fmt.Printf("  role:  %s\n", u.Role)
		logging.Info("user registered", "id", u.ID, "email", u.Email)
		return nil
	},
}

var (
	loginEmail    string
	loginPassword string
	loginTTL      string
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in and receive a JWT",
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginEmail == "" || loginPassword == "" {
			return fmt.Errorf("--email and --password are required")
		}
		u, err := Authenticate(loginEmail, loginPassword)
		if err != nil {
			return err
		}
		ttl := defaultTokenTTL
		if loginTTL != "" {
			d, err := time.ParseDuration(loginTTL)
			if err != nil {
				return fmt.Errorf("invalid --ttl: %w", err)
			}
			ttl = d
		}
		token, exp, err := IssueToken(u, ttl)
		if err != nil {
			return err
		}
		if err := SaveSessionToken(token); err != nil {
			logging.Warn("could not save session token", "error", err)
		}
		fmt.Printf("✓ Signed in as %s\n", u.Email)
		fmt.Printf("  role:    %s\n", u.Role)
		fmt.Printf("  expires: %s\n", exp.Format(time.RFC3339))
		fmt.Println()
		fmt.Println(token)
		logging.Info("user login", "id", u.ID, "email", u.Email)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear local session token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ClearSessionToken(); err != nil {
			return err
		}
		fmt.Println("✓ Signed out")
		return nil
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show identity from the current session token",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := LoadSessionToken()
		if err != nil {
			fmt.Println("Not signed in (no session token).")
			fmt.Println("Run `paradox auth login --email ... --password ...`")
			return nil
		}
		claims, err := ParseToken(token)
		if err != nil {
			fmt.Printf("Session token invalid: %v\n", err)
			fmt.Println("Run `paradox auth login` again.")
			return nil
		}
		fmt.Println("Signed in")
		fmt.Printf("  user_id: %s\n", claims.UserID)
		fmt.Printf("  email:   %s\n", claims.Email)
		fmt.Printf("  role:    %s\n", claims.Role)
		if claims.ExpiresAt != nil {
			fmt.Printf("  expires: %s\n", claims.ExpiresAt.Time.Format(time.RFC3339))
		}
		return nil
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the current session JWT",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := LoadSessionToken()
		if err != nil {
			return fmt.Errorf("no session token — run `paradox auth login` first")
		}
		if _, err := ParseToken(token); err != nil {
			return fmt.Errorf("session token invalid: %w", err)
		}
		fmt.Println(token)
		return nil
	},
}

func init() {
	authRegisterCmd.Flags().StringVar(&registerEmail, "email", "", "user email")
	authRegisterCmd.Flags().StringVar(&registerPassword, "password", "", "password (min 8 chars)")
	authRegisterCmd.Flags().StringVar(&registerRole, "role", "user", "role: user|admin")
	_ = authRegisterCmd.MarkFlagRequired("email")
	_ = authRegisterCmd.MarkFlagRequired("password")

	authLoginCmd.Flags().StringVar(&loginEmail, "email", "", "user email")
	authLoginCmd.Flags().StringVar(&loginPassword, "password", "", "password")
	authLoginCmd.Flags().StringVar(&loginTTL, "ttl", "24h", "token lifetime (e.g. 1h, 24h, 168h)")
	_ = authLoginCmd.MarkFlagRequired("email")
	_ = authLoginCmd.MarkFlagRequired("password")
}

func storePath() (string, error) {
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
