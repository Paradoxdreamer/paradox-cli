package upscale

import "time"

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID           string     `json:"id"`
	InputKey     string     `json:"input_key"`
	InputBucket  string     `json:"input_bucket"`
	OutputKey    string     `json:"output_key,omitempty"`
	OutputBucket string     `json:"output_bucket,omitempty"`
	Scale        int        `json:"scale"`
	Status       Status     `json:"status"`
	Progress     int        `json:"progress"`
	Error        string     `json:"error,omitempty"`
	QueueJobID   string     `json:"queue_job_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

const (
	QueueJobType = "upscale.video"
	DefaultScale = 2
)
