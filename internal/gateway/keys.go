package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type APIKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

type keyStore struct {
	root string
	mu   sync.Mutex
}

func newKeyStore(root string) (*keyStore, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &keyStore{root: root}, nil
}

func (k *keyStore) path() string { return filepath.Join(k.root, "api_keys.json") }

func (k *keyStore) load() ([]APIKey, error) {
	data, err := os.ReadFile(k.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []APIKey
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (k *keyStore) save(list []APIKey) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(k.path(), data, 0o600)
}

func (k *keyStore) Create(name string) (*APIKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	list, err := k.load()
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	secret := "pk_" + hex.EncodeToString(raw)
	key := APIKey{
		ID: fmt.Sprintf("key_%d", time.Now().UnixNano()), Name: name, Key: secret,
		Prefix: secret[:10], CreatedAt: time.Now().UTC(),
	}
	list = append(list, key)
	if err := k.save(list); err != nil {
		return nil, err
	}
	return &key, nil
}

func (k *keyStore) List() ([]APIKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.load()
}

func (k *keyStore) Delete(idOrPrefix string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	list, err := k.load()
	if err != nil {
		return err
	}
	var next []APIKey
	found := false
	for _, item := range list {
		if item.ID == idOrPrefix || item.Prefix == idOrPrefix || item.Key == idOrPrefix {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("api key not found")
	}
	return k.save(next)
}

func (k *keyStore) Validate(secret string) (*APIKey, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	list, err := k.load()
	if err != nil {
		return nil, false
	}
	for i := range list {
		if list[i].Key == secret {
			list[i].LastUsed = time.Now().UTC()
			_ = k.save(list)
			cp := list[i]
			return &cp, true
		}
	}
	return nil, false
}
