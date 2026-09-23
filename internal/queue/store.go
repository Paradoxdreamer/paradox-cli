package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store is a simple file-backed job store.
// Layout:
//
//	<data_dir>/queue/
//	  pending/   <id>.json
//	  running/
//	  succeeded/
//	  failed/
//	  dead/
//
// This is intentionally boring and inspectable. A Redis backend can
// implement the same operations later without changing the CLI.
type Store struct {
	root string
	mu   sync.Mutex
}

func NewStore(root string) (*Store, error) {
	s := &Store{root: root}
	for _, dir := range []string{"pending", "running", "succeeded", "failed", "dead"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) path(status Status, id string) string {
	return filepath.Join(s.root, string(status), id+".json")
}

func (s *Store) write(j *Job) error {
	j.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(j.Status, j.ID), data, 0o644)
}

func (s *Store) read(status Status, id string) (*Job, error) {
	data, err := os.ReadFile(s.path(status, id))
	if err != nil {
		return nil, err
	}
	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

func (s *Store) remove(status Status, id string) error {
	err := os.Remove(s.path(status, id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Enqueue writes a new pending job.
func (s *Store) Enqueue(j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Status = StatusPending
	if j.MaxAttempts <= 0 {
		j.MaxAttempts = DefaultMaxAttempts
	}
	now := time.Now().UTC()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	if j.RunAt.IsZero() {
		j.RunAt = now
	}
	return s.write(j)
}

// ClaimNext picks the oldest pending job that is due and moves it to running.
func (s *Store) ClaimNext() (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.root, string(StatusPending))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	now := time.Now().UTC()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		j, err := s.read(StatusPending, id)
		if err != nil {
			continue
		}
		if j.RunAt.After(now) {
			continue
		}
		if err := s.remove(StatusPending, id); err != nil {
			return nil, err
		}
		j.Status = StatusRunning
		j.Attempts++
		if err := s.write(j); err != nil {
			return nil, err
		}
		return j, nil
	}
	return nil, nil
}

// Complete marks a job succeeded.
func (s *Store) Complete(j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.remove(StatusRunning, j.ID); err != nil {
		return err
	}
	j.Status = StatusSucceeded
	j.LastError = ""
	return s.write(j)
}

// Fail records a failure. If attempts >= max, moves to dead; otherwise pending with backoff.
func (s *Store) Fail(j *Job, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.remove(StatusRunning, j.ID); err != nil {
		return err
	}
	j.LastError = errMsg
	if j.Attempts >= j.MaxAttempts {
		j.Status = StatusDead
		return s.write(j)
	}
	backoff := time.Duration(1<<uint(j.Attempts)) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	j.Status = StatusPending
	j.RunAt = time.Now().UTC().Add(backoff)
	return s.write(j)
}

// List returns jobs in a given status (empty status = all).
func (s *Store) List(status Status, limit int) ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var statuses []Status
	if status == "" {
		statuses = []Status{StatusPending, StatusRunning, StatusSucceeded, StatusFailed, StatusDead}
	} else {
		statuses = []Status{status}
	}

	var out []*Job
	for _, st := range statuses {
		dir := filepath.Join(s.root, string(st))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			id := e.Name()[:len(e.Name())-5]
			j, err := s.read(st, id)
			if err != nil {
				continue
			}
			out = append(out, j)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// Counts returns job counts by status.
func (s *Store) Counts() (map[Status]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := map[Status]int{}
	for _, st := range []Status{StatusPending, StatusRunning, StatusSucceeded, StatusFailed, StatusDead} {
		dir := filepath.Join(s.root, string(st))
		entries, err := os.ReadDir(dir)
		if err != nil {
			counts[st] = 0
			continue
		}
		n := 0
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				n++
			}
		}
		counts[st] = n
	}
	return counts, nil
}

// RequeueDead moves a dead job back to pending.
func (s *Store) RequeueDead(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.read(StatusDead, id)
	if err != nil {
		return fmt.Errorf("dead job %s: %w", id, err)
	}
	if err := s.remove(StatusDead, id); err != nil {
		return err
	}
	j.Status = StatusPending
	j.Attempts = 0
	j.LastError = ""
	j.RunAt = time.Now().UTC()
	return s.write(j)
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
