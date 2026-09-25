package upscale

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/paradox-cloud/paradox/internal/observability"
	"github.com/paradox-cloud/paradox/internal/queue"
	"github.com/paradox-cloud/paradox/internal/storage"
)

// Handler prefers real ffmpeg when input resolves; otherwise simulates.
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
	job.Progress = 5
	_ = store.Put(job)
	_ = observability.Track("upscale.started", map[string]any{"id": id, "scale": job.Scale})

	outBucket := job.OutputBucket
	if outBucket == "" {
		outBucket = "upscale-out"
	}
	workDir, err := os.MkdirTemp("", "paradox-upscale-*")
	if err != nil {
		return failJob(store, job, err)
	}
	defer os.RemoveAll(workDir)

	inPath, source, err := resolveInput(job, workDir)
	if err != nil || !ffmpegAvailable() {
		if !ffmpegAvailable() {
			_ = observability.Track("upscale.ffmpeg_missing", map[string]any{"id": id})
		}
		return simulate(ctx, store, job, outBucket)
	}

	job.Progress = 15
	_ = store.Put(job)
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".mp4"
	}
	outLocal := filepath.Join(workDir, "out"+ext)
	outKey := job.OutputKey
	if outKey == "" {
		outKey = fmt.Sprintf("upscaled/%s_x%d%s", id, job.Scale, ext)
	}
	if err := runFFmpeg(ctx, inPath, outLocal, job.Scale, func(p int) {
		job.Progress = p
		_ = store.Put(job)
		_ = observability.Metric("upscale.progress", float64(p))
	}); err != nil {
		return failJob(store, job, fmt.Errorf("ffmpeg: %w", err))
	}
	job.Progress = 90
	_ = store.Put(job)
	f, err := os.Open(outLocal)
	if err != nil {
		return failJob(store, job, err)
	}
	defer f.Close()
	if err := putObject(outBucket, outKey, f, contentTypeFor(ext), map[string]string{
		"upscale_id": job.ID, "scale": fmt.Sprintf("%d", job.Scale), "engine": "ffmpeg", "source": source,
	}); err != nil {
		return failJob(store, job, err)
	}
	return succeed(store, job, outBucket, outKey, "ffmpeg")
}

func simulate(ctx context.Context, store *Store, job *Job, outBucket string) error {
	for _, p := range []int{10, 25, 50, 75, 90, 100} {
		select {
		case <-ctx.Done():
			return failJob(store, job, ctx.Err())
		default:
		}
		job.Progress = p
		_ = store.Put(job)
		_ = observability.Metric("upscale.progress", float64(p))
		time.Sleep(200 * time.Millisecond)
	}
	outKey := job.OutputKey
	if outKey == "" {
		outKey = fmt.Sprintf("upscaled/%s_x%d.txt", job.ID, job.Scale)
	}
	body := fmt.Sprintf("paradox-upscale\nid=%s\nscale=%dx\ninput=%s/%s\nengine=simulated\n",
		job.ID, job.Scale, job.InputBucket, job.InputKey)
	if err := putObject(outBucket, outKey, strings.NewReader(body), "text/plain", map[string]string{
		"upscale_id": job.ID, "engine": "simulated",
	}); err != nil {
		return failJob(store, job, err)
	}
	return succeed(store, job, outBucket, outKey, "simulated")
}

func succeed(store *Store, job *Job, bucket, key, engine string) error {
	now := time.Now().UTC()
	job.Status = StatusSucceeded
	job.Progress = 100
	job.OutputBucket = bucket
	job.OutputKey = key
	job.FinishedAt = &now
	_ = store.Put(job)
	_ = observability.Track("upscale.completed", map[string]any{
		"id": job.ID, "output": bucket + "/" + key, "engine": engine,
	})
	return nil
}

func failJob(store *Store, job *Job, err error) error {
	job.Status = StatusFailed
	if err != nil {
		job.Error = err.Error()
	}
	_ = store.Put(job)
	_ = observability.Error("upscale failed", map[string]any{"id": job.ID, "error": job.Error})
	return err
}

func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func runFFmpeg(ctx context.Context, in, out string, scale int, onProgress func(int)) error {
	if scale != 2 && scale != 4 {
		scale = 2
	}
	vf := fmt.Sprintf("scale=iw*%d:ih*%d", scale, scale)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", in, "-vf", vf,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-c:a", "copy", "-movflags", "+faststart", out)
	onProgress(25)
	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Run() }()
	t := time.NewTicker(300 * time.Millisecond)
	defer t.Stop()
	p := 25
	for {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
			onProgress(85)
			return nil
		case <-t.C:
			if p < 80 {
				p += 5
				onProgress(p)
			}
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return ctx.Err()
		}
	}
}

func resolveInput(job *Job, workDir string) (localPath, source string, err error) {
	key := job.InputKey
	if st, e := os.Stat(key); e == nil && !st.IsDir() {
		return key, "filesystem", nil
	}
	root, e := storageRoot()
	if e != nil {
		return "", "", e
	}
	st, e := storage.NewStore(root)
	if e != nil {
		return "", "", e
	}
	_, rc, e := st.Get(job.InputBucket, key)
	if e != nil {
		return "", "", fmt.Errorf("input not found on disk or in storage %s/%s: %w", job.InputBucket, key, e)
	}
	defer rc.Close()
	ext := filepath.Ext(key)
	if ext == "" {
		ext = ".bin"
	}
	local := filepath.Join(workDir, "in"+ext)
	f, e := os.Create(local)
	if e != nil {
		return "", "", e
	}
	if _, e = io.Copy(f, rc); e != nil {
		f.Close()
		return "", "", e
	}
	f.Close()
	return local, "storage", nil
}

func putObject(bucket, key string, r io.Reader, contentType string, meta map[string]string) error {
	root, err := storageRoot()
	if err != nil {
		return err
	}
	st, err := storage.NewStore(root)
	if err != nil {
		return err
	}
	_ = st.CreateBucket(bucket)
	_, err = st.Put(bucket, key, r, contentType, meta)
	return err
}

func contentTypeFor(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
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
