package deploy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Release struct {
	ID           string     `json:"id"`
	App          string     `json:"app"`
	Command      string     `json:"command"`
	Dir          string     `json:"dir,omitempty"`
	HealthURL    string     `json:"health_url,omitempty"`
	HealthStatus string     `json:"health_status,omitempty"`
	PID          int        `json:"pid"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	StoppedAt    *time.Time `json:"stopped_at,omitempty"`
	LogFile      string     `json:"log_file,omitempty"`
}

type AppState struct {
	Name    string
	Current *Release
}

type Store struct {
	root string
	mu   sync.Mutex
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func deployRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "deploy"), nil
}

func (s *Store) appDir(app string) string {
	return filepath.Join(s.root, sanitize(app))
}

func (s *Store) Start(app, command, dir, healthURL string, healthWait time.Duration) (*Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app = sanitize(app)
	if app == "" {
		return nil, fmt.Errorf("invalid app name")
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}

	if cur, _ := s.loadCurrent(app); cur != nil && cur.Status == "running" && processAlive(cur.PID) {
		_ = stopPID(cur.PID)
		now := time.Now().UTC()
		cur.Status = "stopped"
		cur.StoppedAt = &now
		_ = s.saveRelease(cur)
	}

	id := fmt.Sprintf("rel_%d", time.Now().UnixNano())
	logPath := filepath.Join(s.appDir(app), "logs", id+".log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	rel := &Release{
		ID: id, App: app, Command: command, Dir: dir, HealthURL: healthURL,
		PID: cmd.Process.Pid, Status: "starting", CreatedAt: time.Now().UTC(), LogFile: logPath,
	}

	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	if healthURL != "" {
		ok := waitHealthy(healthURL, healthWait)
		if ok {
			rel.HealthStatus = "ok"
			rel.Status = "running"
		} else {
			rel.HealthStatus = "failed"
			rel.Status = "failed"
			_ = stopPID(rel.PID)
			now := time.Now().UTC()
			rel.StoppedAt = &now
			_ = s.saveRelease(rel)
			_ = s.saveCurrent(app, rel)
			return rel, fmt.Errorf("health check failed for %s (process stopped)", healthURL)
		}
	} else {
		time.Sleep(200 * time.Millisecond)
		if !processAlive(rel.PID) {
			rel.Status = "failed"
			_ = s.saveRelease(rel)
			_ = s.saveCurrent(app, rel)
			return rel, fmt.Errorf("process exited immediately — check: paradox deploy logs --name %s", app)
		}
		rel.Status = "running"
	}

	if err := s.saveRelease(rel); err != nil {
		return nil, err
	}
	if err := s.saveCurrent(app, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

func (s *Store) Stop(app string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	app = sanitize(app)
	cur, err := s.loadCurrent(app)
	if err != nil || cur == nil {
		return fmt.Errorf("no current release for %q", app)
	}
	if processAlive(cur.PID) {
		_ = stopPID(cur.PID)
	}
	now := time.Now().UTC()
	cur.Status = "stopped"
	cur.StoppedAt = &now
	_ = s.saveRelease(cur)
	return s.saveCurrent(app, cur)
}

func (s *Store) Rollback(app string) (*Release, error) {
	s.mu.Lock()
	app = sanitize(app)
	rels, err := s.listReleasesUnlocked(app)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	cur, _ := s.loadCurrent(app)
	var prev *Release
	for i := len(rels) - 1; i >= 0; i-- {
		r := rels[i]
		if cur != nil && r.ID == cur.ID {
			continue
		}
		if r.Command != "" {
			prev = r
			break
		}
	}
	if prev == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("no previous release to roll back to")
	}
	if cur != nil && processAlive(cur.PID) {
		_ = stopPID(cur.PID)
		now := time.Now().UTC()
		cur.Status = "stopped"
		cur.StoppedAt = &now
		_ = s.saveRelease(cur)
	}
	cmd, dir, health := prev.Command, prev.Dir, prev.HealthURL
	s.mu.Unlock()
	return s.Start(app, cmd, dir, health, 15*time.Second)
}

func (s *Store) ListApps() ([]AppState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []AppState
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cur, _ := s.loadCurrent(e.Name())
		out = append(out, AppState{Name: e.Name(), Current: cur})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) ListReleases(app string) ([]*Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listReleasesUnlocked(app)
}

func (s *Store) listReleasesUnlocked(app string) ([]*Release, error) {
	var apps []string
	if app != "" {
		apps = []string{sanitize(app)}
	} else {
		entries, err := os.ReadDir(s.root)
		if err != nil {
			return nil, nil
		}
		for _, e := range entries {
			if e.IsDir() {
				apps = append(apps, e.Name())
			}
		}
	}
	var out []*Release
	for _, a := range apps {
		dir := filepath.Join(s.appDir(a), "releases")
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var r Release
			if json.Unmarshal(data, &r) == nil {
				out = append(out, &r)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) Logs(app string, lines int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	app = sanitize(app)
	cur, err := s.loadCurrent(app)
	if err != nil || cur == nil || cur.LogFile == "" {
		return "", fmt.Errorf("no logs for %q", app)
	}
	data, err := os.ReadFile(cur.LogFile)
	if err != nil {
		return "", err
	}
	if lines <= 0 {
		return string(data), nil
	}
	all := strings.Split(string(data), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n") + "\n", nil
}

func (s *Store) saveRelease(r *Release) error {
	dir := filepath.Join(s.appDir(r.App), "releases")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, r.ID+".json"), data, 0o644)
}

func (s *Store) saveCurrent(app string, r *Release) error {
	dir := s.appDir(app)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "current.json"), data, 0o644)
}

func (s *Store) loadCurrent(app string) (*Release, error) {
	data, err := os.ReadFile(filepath.Join(s.appDir(app), "current.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func stopPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	if processAlive(pid) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	return nil
}

func waitHealthy(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return true
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return false
}

func sanitize(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
