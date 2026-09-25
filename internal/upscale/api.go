package upscale

import (
	"fmt"
	"time"

	"github.com/paradox-cloud/paradox/internal/observability"
	"github.com/paradox-cloud/paradox/internal/queue"
)

type SubmitRequest struct {
	Input  string `json:"input"`
	Bucket string `json:"bucket,omitempty"`
	Scale  int    `json:"scale,omitempty"`
}

func Submit(req SubmitRequest) (*Job, error) {
	if req.Input == "" {
		return nil, fmt.Errorf("input is required")
	}
	scale := req.Scale
	if scale == 0 {
		scale = DefaultScale
	}
	if scale != 2 && scale != 4 {
		return nil, fmt.Errorf("scale must be 2 or 4")
	}
	bucket := req.Bucket
	if bucket == "" {
		bucket = "upscale-in"
	}
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("up_%d", time.Now().UnixNano())
	job := &Job{
		ID: id, InputKey: req.Input, InputBucket: bucket, Scale: scale,
		Status: StatusQueued, Progress: 0, CreatedAt: time.Now().UTC(),
	}
	if err := store.Put(job); err != nil {
		return nil, err
	}
	qstore, err := openQueue()
	if err != nil {
		return nil, err
	}
	qjID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	qj := &queue.Job{
		ID: qjID, Type: QueueJobType,
		Payload: map[string]any{"upscale_id": id, "scale": scale, "input": bucket + "/" + req.Input},
		MaxAttempts: 3,
	}
	if err := qstore.Enqueue(qj); err != nil {
		return nil, err
	}
	job.QueueJobID = qjID
	_ = store.Put(job)
	_ = observability.Track("upscale.submitted", map[string]any{"id": id, "scale": scale})
	return job, nil
}

func Get(id string) (*Job, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	return store.Get(id)
}

func ListJobs(limit int) ([]*Job, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	return store.List(limit)
}
