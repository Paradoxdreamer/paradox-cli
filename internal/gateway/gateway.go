package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/paradox-cloud/paradox/internal/flags"
	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/queue"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name: "gateway", Description: "API Gateway — HTTP entry, API keys, rate limits",
		ConfigSection: "gateway", Commands: []*cobra.Command{gatewayCmd},
		DoctorChecks: []registry.DoctorCheck{{Name: "Gateway data", Fn: checkGateway}},
	})
	gatewayCmd.AddCommand(gatewayInitCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)
	gatewayCmd.AddCommand(gatewayKeysCmd)
	gatewayKeysCmd.AddCommand(gatewayKeysCreateCmd)
	gatewayKeysCmd.AddCommand(gatewayKeysListCmd)
	gatewayKeysCmd.AddCommand(gatewayKeysDeleteCmd)
}

var gatewayCmd = &cobra.Command{
	Use: "gateway", Short: "Paradox API Gateway",
	Long: `HTTP front door for Paradox services.

  paradox gateway init
  paradox gateway keys create --name local
  paradox gateway start [--addr :8080] [--require-key]

Endpoints:
  GET  /health
  GET  /v1/whoami              Authorization: Bearer <JWT>
  GET  /v1/flags/eval?key=…   X-API-Key: pk_…
  POST /v1/queue/enqueue      X-API-Key: pk_…
  GET  /v1/queue/status       X-API-Key: pk_…
`,
}

var gatewayInitCmd = &cobra.Command{
	Use: "init", Short: "Initialize gateway data dir",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := gatewayRoot()
		if err != nil {
			return err
		}
		if _, err := newKeyStore(root); err != nil {
			return err
		}
		fmt.Printf("✓ Gateway data ready at %s\n", root)
		return nil
	},
}

var startAddr string
var startRequireKey bool
var startRPS int

var gatewayStartCmd = &cobra.Command{
	Use: "start", Short: "Start the API Gateway HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := gatewayRoot()
		if err != nil {
			return err
		}
		ks, err := newKeyStore(root)
		if err != nil {
			return err
		}
		srv := &Server{Addr: startAddr, Keys: ks, Limiter: newLimiter(startRPS, startRPS*2), RequireKey: startRequireKey}
		httpSrv := &http.Server{Addr: srv.Addr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(shutdownCtx)
		}()
		fmt.Printf("Paradox Gateway listening on http://%s\n", addrHost(srv.Addr))
		if startRequireKey {
			fmt.Println("API key required (X-API-Key or Bearer pk_…)")
		} else {
			fmt.Println("API key optional (rate limit by IP)")
		}
		fmt.Println("Endpoints: /health /v1/whoami /v1/flags/eval /v1/queue/enqueue /v1/queue/status")
		fmt.Println("Ctrl+C to stop")
		logging.Info("gateway starting", "addr", srv.Addr, "require_key", startRequireKey)
		err = httpSrv.ListenAndServe()
		if err == http.ErrServerClosed {
			fmt.Println("Gateway stopped")
			return nil
		}
		return err
	},
}

var gatewayKeysCmd = &cobra.Command{Use: "keys", Short: "Manage API keys"}
var keyName string

var gatewayKeysCreateCmd = &cobra.Command{
	Use: "create", Short: "Create an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		if keyName == "" {
			keyName = "default"
		}
		root, err := gatewayRoot()
		if err != nil {
			return err
		}
		ks, err := newKeyStore(root)
		if err != nil {
			return err
		}
		k, err := ks.Create(keyName)
		if err != nil {
			return err
		}
		fmt.Println("✓ API key created")
		fmt.Printf("  id:     %s\n", k.ID)
		fmt.Printf("  name:   %s\n", k.Name)
		fmt.Printf("  key:    %s\n", k.Key)
		fmt.Printf("  prefix: %s\n", k.Prefix)
		return nil
	},
}

var gatewayKeysListCmd = &cobra.Command{
	Use: "list", Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := gatewayRoot()
		if err != nil {
			return err
		}
		ks, err := newKeyStore(root)
		if err != nil {
			return err
		}
		list, err := ks.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("(no keys — run: paradox gateway keys create --name local)")
			return nil
		}
		fmt.Printf("%-20s %-12s %s\n", "ID", "PREFIX", "NAME")
		for _, k := range list {
			fmt.Printf("%-20s %-12s %s\n", k.ID, k.Prefix+"…", k.Name)
		}
		return nil
	},
}

var keyDeleteID string

var gatewayKeysDeleteCmd = &cobra.Command{
	Use: "delete", Short: "Delete an API key by id or prefix",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := keyDeleteID
		if id == "" && len(args) > 0 {
			id = args[0]
		}
		if id == "" {
			return fmt.Errorf("provide key id or prefix")
		}
		root, err := gatewayRoot()
		if err != nil {
			return err
		}
		ks, err := newKeyStore(root)
		if err != nil {
			return err
		}
		if err := ks.Delete(id); err != nil {
			return err
		}
		fmt.Printf("✓ Deleted key %s\n", id)
		return nil
	},
}

func init() {
	gatewayStartCmd.Flags().StringVar(&startAddr, "addr", "127.0.0.1:8080", "listen address")
	gatewayStartCmd.Flags().BoolVar(&startRequireKey, "require-key", false, "require X-API-Key")
	gatewayStartCmd.Flags().IntVar(&startRPS, "rps", 30, "rate limit per key/IP")
	gatewayKeysCreateCmd.Flags().StringVar(&keyName, "name", "default", "key name")
	gatewayKeysDeleteCmd.Flags().StringVar(&keyDeleteID, "id", "", "key id or prefix")
}

func gatewayRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "gateway"), nil
}

func checkGateway() (bool, string) {
	root, err := gatewayRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox gateway init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, "not a directory"
	}
	return true, root
}

func addrHost(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "127.0.0.1" + addr
	}
	return addr
}

func openFlagsStore() (*flags.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".paradox", "flags")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = filepath.Join(d, "flags")
	}
	return flags.NewStore(base)
}

func openQueueStore() (*queue.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".paradox", "queue")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = filepath.Join(d, "queue")
	}
	return queue.NewStore(base)
}
