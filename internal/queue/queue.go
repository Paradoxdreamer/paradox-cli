package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name:          "queue",
		Description:   "Background jobs — enqueue, workers, retry, dead-letter",
		ConfigSection: "queue",
		Commands:      []*cobra.Command{queueCmd},
		DoctorChecks: []registry.DoctorCheck{
			{Name: "Queue store", Fn: checkQueueStore},
		},
	})

	queueCmd.AddCommand(queueStatusCmd)
	queueCmd.AddCommand(queueInitCmd)
	queueCmd.AddCommand(queueEnqueueCmd)
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queueWorkerCmd)
	queueCmd.AddCommand(queueRequeueCmd)
}

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Paradox Queue — background jobs",
	Long: `Paradox Queue: enqueue → worker → retry → dead-letter.

  paradox queue init                 Create local queue store
  paradox queue status               Counts by status
  paradox queue enqueue --type T     Enqueue a job
  paradox queue list [--status S]    List jobs
  paradox queue worker               Run a worker process
  paradox queue requeue <id>         Move a dead job back to pending

Default backend is file-based under ~/.paradox/queue (inspectable, no Redis required).
`,
}

var queueInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the local queue store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := queueRoot()
		if err != nil {
			return err
		}
		if _, err := NewStore(root); err != nil {
			return err
		}
		fmt.Printf("✓ Queue store ready at %s\n", root)
		logging.Info("queue store initialized", "path", root)
		return nil
	},
}

var queueStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show queue counts by status",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		counts, err := store.Counts()
		if err != nil {
			return err
		}
		fmt.Println("Paradox Queue")
		fmt.Println("=============")
		root, _ := queueRoot()
		fmt.Printf("  Store:      %s\n", root)
		fmt.Printf("  pending:    %d\n", counts[StatusPending])
		fmt.Printf("  running:    %d\n", counts[StatusRunning])
		fmt.Printf("  succeeded:  %d\n", counts[StatusSucceeded])
		fmt.Printf("  failed:     %d\n", counts[StatusFailed])
		fmt.Printf("  dead:       %d\n", counts[StatusDead])
		return nil
	},
}

var (
	enqueueType    string
	enqueuePayload string
	enqueueMax     int
)

var queueEnqueueCmd = &cobra.Command{
	Use:   "enqueue",
	Short: "Enqueue a job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if enqueueType == "" {
			return fmt.Errorf("--type is required")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		payload := map[string]any{}
		if enqueuePayload != "" {
			if err := json.Unmarshal([]byte(enqueuePayload), &payload); err != nil {
				return fmt.Errorf("invalid --payload JSON: %w", err)
			}
		}
		id := fmt.Sprintf("job_%d", time.Now().UnixNano())
		j := &Job{
			ID:          id,
			Type:        enqueueType,
			Payload:     payload,
			MaxAttempts: enqueueMax,
		}
		if err := store.Enqueue(j); err != nil {
			return err
		}
		fmt.Printf("✓ Enqueued %s (type=%s)\n", id, enqueueType)
		logging.Info("job enqueued", "id", id, "type", enqueueType)
		return nil
	},
}

var listStatus string
var listLimit int

var queueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		var st Status
		if listStatus != "" {
			st = Status(listStatus)
		}
		jobs, err := store.List(st, listLimit)
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			fmt.Println("(no jobs)")
			return nil
		}
		fmt.Printf("%-24s %-12s %-20s %s\n", "ID", "STATUS", "TYPE", "ATTEMPTS")
		for _, j := range jobs {
			fmt.Printf("%-24s %-12s %-20s %d/%d\n", j.ID, j.Status, j.Type, j.Attempts, j.MaxAttempts)
			if j.LastError != "" {
				fmt.Printf("  error: %s\n", j.LastError)
			}
		}
		return nil
	},
}

var workerInterval string

var queueWorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Run a queue worker (blocks until Ctrl+C)",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		interval := 500 * time.Millisecond
		if workerInterval != "" {
			d, err := time.ParseDuration(workerInterval)
			if err != nil {
				return fmt.Errorf("invalid --interval: %w", err)
			}
			interval = d
		}

		handlers := map[string]Handler{
			"echo": func(ctx context.Context, j *Job) error {
				logging.Info("echo handler", "payload", j.Payload)
				fmt.Printf("[echo] job %s payload=%v\n", j.ID, j.Payload)
				return nil
			},
			"fail": func(ctx context.Context, j *Job) error {
				return fmt.Errorf("intentional failure for demo")
			},
		}

		w := &Worker{
			Store:    store,
			Handlers: handlers,
			Interval: interval,
			DefaultHandler: func(ctx context.Context, j *Job) error {
				logging.Info("default handler (no-op)", "type", j.Type, "id", j.ID)
				fmt.Printf("[default] processed %s type=%s\n", j.ID, j.Type)
				return nil
			},
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Println("Worker running (Ctrl+C to stop)…")
		err = w.Run(ctx)
		if err == context.Canceled {
			return nil
		}
		return err
	},
}

var queueRequeueCmd = &cobra.Command{
	Use:   "requeue <job-id>",
	Short: "Move a dead job back to pending",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		if err := store.RequeueDead(args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ Requeued %s\n", args[0])
		return nil
	},
}

func init() {
	queueEnqueueCmd.Flags().StringVar(&enqueueType, "type", "", "job type (e.g. echo, email.send)")
	queueEnqueueCmd.Flags().StringVar(&enqueuePayload, "payload", "{}", "JSON payload")
	queueEnqueueCmd.Flags().IntVar(&enqueueMax, "max-attempts", DefaultMaxAttempts, "max attempts before dead-letter")
	_ = queueEnqueueCmd.MarkFlagRequired("type")

	queueListCmd.Flags().StringVar(&listStatus, "status", "", "filter: pending|running|succeeded|failed|dead")
	queueListCmd.Flags().IntVar(&listLimit, "limit", 50, "max jobs to show")

	queueWorkerCmd.Flags().StringVar(&workerInterval, "interval", "500ms", "poll interval")
}

func openStore() (*Store, error) {
	root, err := queueRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func checkQueueStore() (ok bool, detail string) {
	root, err := queueRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox queue init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, root + " is not a directory"
	}
	return true, root
}
