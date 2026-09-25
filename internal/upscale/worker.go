package upscale

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paradox-cloud/paradox/internal/observability"
	"github.com/paradox-cloud/paradox/internal/queue"
	"github.com/paradox-cloud/paradox/internal/storage"
)

func Handler(ctx context.Context, qj *queue.Job) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	id, _ := qj.Payload["upscale_id"].(string)
	if id == "" {
		return fmt.Errorf("payload missing upscale_id")
	}
	job, err := store.Get(id)
	if err != nil {
		return err
	}
	job.Status = StatusRunning
	job.Progress = 0
	_ = store.Put(job)
	_ = observability.Track("upscale.started", map[string]any{"id": id, "scale": job.Scale})

	for _, p := range []int{10, 25, 50, 75, 90, 100} {
		select {
		case <-ctx.Done():
			job.Status = StatusFailed
			job.Error = "cancelled"
			_ = store.Put(job)
			return ctx.Err()
		default:
		}
		job.Progress = p
		_ = store.Put(job)
		_ = observability.Metric("upscale.progress", float64(p))
		time.Sleep(200 * time.Millisecond)
	}

	outBucket := job.OutputBucket
	if outBucket == "" {
		outBucket = "upscale-out"
	}
	outKey := job.OutputKey
	if outKey == "" {
		outKey = fmt.Sprintf("upscaled/%s_x%d.txt", id, job.Scale)
	}
	if err := writeOutput(outBucket, outKey, job); err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		_ = store.Put(job)
		_ = observability.Error("upscale failed", map[string]any{"id": id, "error": err.Error()})
		return err
	}
	now := time.Now().UTC()
	job.Status = StatusSucceeded
	job.Progress = 100
	job.OutputBucket = outBucket
	job.OutputKey = outKey
	job.FinishedAt = &now
	_ = store.Put(job)
	_ = observability.Track("upscale.completed", map[string]any{"id": id, "output": outBucket + "/" + outKey})
	return nil
}

func writeOutput(bucket, key string, job *Job) error {
	root, err := storageRoot()
	if err != nil {
		return err
	}
	st, err := storage.NewStore(root)
	if err != nil {
		return err
	}
	_ = st.CreateBucket(bucket)
	body := fmt.Sprintf("paradox-upscale\nid=%s\nscale=%dx\ninput=%s/%s\nsimulated=true\n",
		job.ID, job.Scale, job.InputBucket, job.InputKey)
	_, err = st.Put(bucket, key, strings.NewReader(body), "text/plain", map[string]string{"upscale_id": job.ID})
	return err
}

func openStore() (*Store, error) {
	root, err := upscaleRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func storageRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "storage"), nil
}
