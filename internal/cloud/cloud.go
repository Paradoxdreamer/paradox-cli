package cloud

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/paradox-cloud/paradox/internal/gateway"
	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/queue"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/paradox-cloud/paradox/internal/upscale"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name: "cloud", Description: "Local Paradox Cloud — gateway + workers in one process",
		ConfigSection: "cloud", Commands: []*cobra.Command{cloudCmd},
	})
	cloudCmd.AddCommand(cloudUpCmd, cloudStatusCmd)
}

var cloudCmd = &cobra.Command{
	Use: "cloud", Short: "Local Paradox Cloud",
	Long: `Run the platform pieces together — one process, one Ctrl+C.

  paradox cloud up [--addr 127.0.0.1:8080] [--require-key]
  paradox cloud status

Starts API Gateway + queue worker (echo + upscale.video).
`,
}

var cloudAddr string
var cloudRequireKey bool
var cloudRPS int

var cloudUpCmd = &cobra.Command{
	Use: "up", Short: "Start gateway + workers (local cloud)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCloudUp(cloudAddr, cloudRequireKey, cloudRPS)
	},
}

var cloudStatusCmd = &cobra.Command{
	Use: "status", Short: "Probe local cloud health",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := cloudAddr
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		url := "http://" + gateway.AddrHost(addr) + "/health"
		client := &http.Client{Timeout: 2 * time.Second}
		res, err := client.Get(url)
		if err != nil {
			fmt.Printf("Gateway: down (%v)\n", err)
			fmt.Println("Start with: paradox cloud up")
			return nil
		}
		defer res.Body.Close()
		fmt.Printf("Gateway: up  %s → HTTP %d\n", url, res.StatusCode)
		if qroot, err := dataPath("queue"); err == nil {
			if st, err := queue.NewStore(qroot); err == nil {
				if c, err := st.Counts(); err == nil {
					fmt.Printf("Queue:   pending=%d running=%d succeeded=%d failed=%d dead=%d\n",
						c[queue.StatusPending], c[queue.StatusRunning], c[queue.StatusSucceeded],
						c[queue.StatusFailed], c[queue.StatusDead])
				}
			}
		}
		return nil
	},
}

func init() {
	cloudUpCmd.Flags().StringVar(&cloudAddr, "addr", "127.0.0.1:8080", "gateway listen address")
	cloudUpCmd.Flags().BoolVar(&cloudRequireKey, "require-key", false, "require API key")
	cloudUpCmd.Flags().IntVar(&cloudRPS, "rps", 30, "rate limit")
	cloudStatusCmd.Flags().StringVar(&cloudAddr, "addr", "127.0.0.1:8080", "gateway address to probe")
}

func runCloudUp(addr string, requireKey bool, rps int) error {
	if _, err := gateway.EnsureReady(); err != nil {
		return fmt.Errorf("gateway init: %w", err)
	}
	qroot, err := dataPath("queue")
	if err != nil {
		return err
	}
	qstore, err := queue.NewStore(qroot)
	if err != nil {
		return err
	}
	if uroot, err := dataPath("upscale"); err == nil {
		_, _ = upscale.NewStore(uroot)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║         PARADOX CLOUD (local)            ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("  Gateway  http://%s\n", gateway.AddrHost(addr))
	if requireKey {
		fmt.Println("  Auth     API key required")
	} else {
		fmt.Println("  Auth     API key optional")
	}
	fmt.Println("  Workers  queue (echo) + upscale.video")
	fmt.Println("  Stop     Ctrl+C")
	fmt.Println()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := gateway.RunServer(ctx, addr, requireKey, rps); err != nil {
			errCh <- fmt.Errorf("gateway: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		w := &queue.Worker{
			Store: qstore, Interval: 500 * time.Millisecond,
			Handlers: map[string]queue.Handler{
				"echo":               echoHandler,
				upscale.QueueJobType: upscale.Handler,
			},
			DefaultHandler: func(ctx context.Context, j *queue.Job) error {
				return fmt.Errorf("no handler for job type %q", j.Type)
			},
		}
		logging.Info("cloud worker started")
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("worker: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down Paradox Cloud…")
	case err := <-errCh:
		stop()
		wg.Wait()
		return err
	}
	wg.Wait()
	fmt.Println("Paradox Cloud stopped")
	return nil
}

func echoHandler(ctx context.Context, j *queue.Job) error {
	logging.Info("echo handler", "id", j.ID, "payload", j.Payload)
	fmt.Printf("[echo] job %s payload=%v\n", j.ID, j.Payload)
	return nil
}

func dataPath(svc string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	p := filepath.Join(base, svc)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}
