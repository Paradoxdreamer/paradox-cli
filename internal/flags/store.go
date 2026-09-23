package flags

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store is a file-backed flag store.
// Layout: <data_dir>/flags/<key>.json
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

func (s *Store) path(key string) string {
	return filepath.Join(s.root, key+".json")
}

func (s *Store) Get(key string) (*Flag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("flag %q not found", key)
		}
		return nil, err
	}
	var f Flag
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) Set(f *Flag) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.Key == "" {
		return fmt.Errorf("flag key is required")
	}
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(f.Key), data, 0o644)
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(key))
	if os.IsNotExist(err) {
		return fmt.Errorf("flag %q not found", key)
	}
	return err
}

func (s *Store) List() ([]*Flag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []*Flag
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		key := e.Name()[:len(e.Name())-5]
		data, err := os.ReadFile(s.path(key))
		if err != nil {
			continue
		}
		var f Flag
		if err := json.Unmarshal(data, &f); err != nil {
			continue
		}
		out = append(out, &f)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func flagsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "flags"), nil
}
