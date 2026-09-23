package queue

import (
	"time"
)

// Status is the lifecycle state of a job.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusDead      Status = "dead" // moved to dead-letter after max retries
)

// Job is a unit of work in the queue.
type Job struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // e.g. "email.send", "image.resize"
	Payload     map[string]any    `json:"payload"`
	Status      Status            `json:"status"`
	Attempts    int               `json:"attempts"`
	MaxAttempts int               `json:"max_attempts"`
	LastError   string            `json:"last_error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	RunAt       time.Time         `json:"run_at"` // for delayed / retry backoff
	Meta        map[string]string `json:"meta,omitempty"`
}

// DefaultMaxAttempts is used when enqueue does not specify one.
const DefaultMaxAttempts = 3
