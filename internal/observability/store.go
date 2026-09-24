package observability

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Event struct {
	Time    time.Time      `json:"time"`
	Type    string         `json:"type"`
	Name    string         `json:"name,omitempty"`
	Message string         `json:"message,omitempty"`
	Level   string         `json:"level,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Service string         `json:"service,omitempty"`
}

type MetricPoint struct {
	Time  time.Time `json:"time"`
	Name  string    `json:"name"`
	Value float64   `json:"value"`
	Unit  string    `json:"unit,omitempty"`
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

func (s *Store) eventsPath() string {
	return filepath.Join(s.root, "events-"+time.Now().UTC().Format("2006-01-02")+".jsonl")
}
func (s *Store) metricsPath() string {
	return filepath.Join(s.root, "metrics-"+time.Now().UTC().Format("2006-01-02")+".jsonl")
}

func (s *Store) AppendEvent(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	return appendJSONL(s.eventsPath(), e)
}

func (s *Store) AppendMetric(m MetricPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.Time.IsZero() {
		m.Time = time.Now().UTC()
	}
	return appendJSONL(s.metricsPath(), m)
}

func appendJSONL(path string, v any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}

func (s *Store) TailEvents(n int, typeFilter string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, _ := filepath.Glob(filepath.Join(s.root, "events-*.jsonl"))
	sort.Strings(paths)
	var all []Event
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var e Event
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			if typeFilter != "" && e.Type != typeFilter {
				continue
			}
			all = append(all, e)
		}
		_ = f.Close()
	}
	if n > 0 && len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

type MetricSummary struct {
	Name  string  `json:"name"`
	Count int     `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Last  float64 `json:"last"`
}

func (s *Store) SummarizeMetrics() ([]MetricSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, _ := filepath.Glob(filepath.Join(s.root, "metrics-*.jsonl"))
	sort.Strings(paths)
	agg := map[string]*MetricSummary{}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var m MetricPoint
			if json.Unmarshal(sc.Bytes(), &m) != nil {
				continue
			}
			a, ok := agg[m.Name]
			if !ok {
				agg[m.Name] = &MetricSummary{Name: m.Name, Count: 1, Sum: m.Value, Min: m.Value, Max: m.Value, Last: m.Value}
				continue
			}
			a.Count++
			a.Sum += m.Value
			if m.Value < a.Min {
				a.Min = m.Value
			}
			if m.Value > a.Max {
				a.Max = m.Value
			}
			a.Last = m.Value
		}
		_ = f.Close()
	}
	var out []MetricSummary
	for _, v := range agg {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) Counts() (events, metrics, errors int, err error) {
	ev, err := s.TailEvents(0, "")
	if err != nil {
		return 0, 0, 0, err
	}
	events = len(ev)
	for _, e := range ev {
		if e.Type == "error" {
			errors++
		}
	}
	sums, err := s.SummarizeMetrics()
	if err != nil {
		return events, 0, errors, err
	}
	for _, m := range sums {
		metrics += m.Count
	}
	return events, metrics, errors, nil
}

func obsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "observability"), nil
}

func DefaultStore() (*Store, error) {
	root, err := obsRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func Track(name string, attrs map[string]any) error {
	s, err := DefaultStore()
	if err != nil {
		return err
	}
	return s.AppendEvent(Event{Type: "track", Name: name, Attrs: attrs, Service: "cli"})
}

func Error(msg string, attrs map[string]any) error {
	s, err := DefaultStore()
	if err != nil {
		return err
	}
	return s.AppendEvent(Event{Type: "error", Message: msg, Level: "error", Attrs: attrs, Service: "cli"})
}

func Metric(name string, value float64) error {
	s, err := DefaultStore()
	if err != nil {
		return err
	}
	return s.AppendMetric(MetricPoint{Name: name, Value: value})
}
