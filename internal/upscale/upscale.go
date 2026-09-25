package upscale

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/observability"
	"github.com/paradox-cloud/paradox/internal/queue"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name: "upscale", Description: "Video upscale platform — jobs, progress, workers",
		ConfigSection: "upscale", Commands: []*cobra.Command{upscaleCmd},
		DoctorChecks: []registry.DoctorCheck{{Name: "Upscale store", Fn: checkUpscale}},
	})
	upscaleCmd.AddCommand(upscaleInitCmd, upscaleSubmitCmd, upscaleStatusCmd, upscaleListCmd, upscaleWorkerCmd)
}

var upscaleCmd = &cobra.Command{
	Use: "upscale", Short: "Video upscale platform (distributed media jobs)",
	Long: `Production-shaped video upscale — platform not the ML model.

  paradox upscale submit --input videos/clip.mp4 --scale 2
  paradox upscale worker
  paradox upscale status <id>
`,
}

var upscaleInitCmd = &cobra.Command{
	Use: "init", Short: "Initialize upscale store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := upscaleRoot()
		if err != nil {
			return err
		}
		if _, err := NewStore(root); err != nil {
			return err
		}
		qroot, err := queueRoot()
		if err != nil {
			return err
		}
		if _, err := queue.NewStore(qroot); err != nil {
			return err
		}
		fmt.Printf("✓ Upscale platform ready at %s\n", root)
		logging.Info("upscale initialized", "path", root)
		return nil
	},
}

var submitInput, submitBucket string
var submitScale int

var upscaleSubmitCmd = &cobra.Command{
	Use: "submit", Short: "Submit an upscale job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if submitInput == "" {
			return fmt.Errorf("--input is required")
		}
		if submitScale != 2 && submitScale != 4 {
			return fmt.Errorf("--scale must be 2 or 4")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		id := fmt.Sprintf("up_%d", time.Now().UnixNano())
		bucket := submitBucket
		if bucket == "" {
			bucket = "upscale-in"
		}
		job := &Job{ID: id, InputKey: submitInput, InputBucket: bucket, Scale: submitScale, Status: StatusQueued, CreatedAt: time.Now().UTC()}
		if err := store.Put(job); err != nil {
			return err
		}
		qstore, err := openQueue()
		if err != nil {
			return err
		}
		qjID := fmt.Sprintf("job_%d", time.Now().UnixNano())
		qj := &queue.Job{ID: qjID, Type: QueueJobType, Payload: map[string]any{"upscale_id": id, "scale": submitScale}, MaxAttempts: 3}
		if err := qstore.Enqueue(qj); err != nil {
			return err
		}
		job.QueueJobID = qjID
		_ = store.Put(job)
		_ = observability.Track("upscale.submitted", map[string]any{"id": id})
		fmt.Printf("✓ Submitted %s\n  input: %s/%s\n  scale: %dx\n  Run: paradox upscale worker\n", id, bucket, submitInput, submitScale)
		return nil
	},
}

var upscaleStatusCmd = &cobra.Command{
	Use: "status <id>", Short: "Show job progress", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		j, err := store.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("  id: %s\n  status: %s\n  progress: %d%%\n  scale: %dx\n  input: %s/%s\n", j.ID, j.Status, j.Progress, j.Scale, j.InputBucket, j.InputKey)
		if j.OutputKey != "" {
			fmt.Printf("  output: %s/%s\n", j.OutputBucket, j.OutputKey)
		}
		if j.Error != "" {
			fmt.Printf("  error: %s\n", j.Error)
		}
		return nil
	},
}

var listLimit int
var upscaleListCmd = &cobra.Command{
	Use: "list", Short: "List recent jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		jobs, err := store.List(listLimit)
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			fmt.Println("(no jobs)")
			return nil
		}
		fmt.Printf("%-22s %-10s %4s %s\n", "ID", "STATUS", "PCT", "INPUT")
		for _, j := range jobs {
			fmt.Printf("%-22s %-10s %3d%% %s/%s\n", j.ID, j.Status, j.Progress, j.InputBucket, j.InputKey)
		}
		return nil
	},
}

var workerInterval string
var upscaleWorkerCmd = &cobra.Command{
	Use: "worker", Short: "Process upscale.video queue jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		qstore, err := openQueue()
		if err != nil {
			return err
		}
		interval := 500 * time.Millisecond
		if workerInterval != "" {
			if d, err := time.ParseDuration(workerInterval); err == nil {
				interval = d
			}
		}
		w := &queue.Worker{
			Store: qstore, Interval: interval,
			Handlers: map[string]queue.Handler{QueueJobType: Handler},
			DefaultHandler: func(ctx context.Context, j *queue.Job) error {
				logging.Info("skip non-upscale job", "type", j.Type)
				return nil
			},
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Println("Upscale worker running (Ctrl+C)…")
		err = w.Run(ctx)
		if err == context.Canceled {
			return nil
		}
		return err
	},
}

func init() {
	upscaleSubmitCmd.Flags().StringVar(&submitInput, "input", "", "input object key")
	upscaleSubmitCmd.Flags().StringVar(&submitBucket, "bucket", "upscale-in", "input bucket")
	upscaleSubmitCmd.Flags().IntVar(&submitScale, "scale", DefaultScale, "2 or 4")
	_ = upscaleSubmitCmd.MarkFlagRequired("input")
	upscaleListCmd.Flags().IntVar(&listLimit, "limit", 20, "max jobs")
	upscaleWorkerCmd.Flags().StringVar(&workerInterval, "interval", "500ms", "poll interval")
}

func openQueue() (*queue.Store, error) {
	root, err := queueRoot()
	if err != nil {
		return nil, err
	}
	return queue.NewStore(root)
}

func queueRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "queue"), nil
}

func checkUpscale() (bool, string) {
	root, err := upscaleRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox upscale init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, "not a directory"
	}
	return true, root
}
