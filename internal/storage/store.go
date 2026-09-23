package storage

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ObjectMeta struct {
	Key         string            `json:"key"`
	Bucket      string            `json:"bucket"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type,omitempty"`
	ETag        string            `json:"etag"`
	UploadedAt  time.Time         `json:"uploaded_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type BucketInfo struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	s := &Store{root: root}
	if _, err := s.loadBuckets(); err != nil {
		if err := s.saveBuckets(nil); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) bucketsPath() string { return filepath.Join(s.root, "buckets.json") }

func (s *Store) loadBuckets() ([]BucketInfo, error) {
	data, err := os.ReadFile(s.bucketsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []BucketInfo
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) saveBuckets(list []BucketInfo) error {
	if list == nil {
		list = []BucketInfo{}
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.bucketsPath(), data, 0o644)
}

func (s *Store) CreateBucket(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = sanitizeBucket(name)
	if name == "" {
		return fmt.Errorf("invalid bucket name")
	}
	list, err := s.loadBuckets()
	if err != nil {
		return err
	}
	for _, b := range list {
		if b.Name == name {
			return fmt.Errorf("bucket %q already exists", name)
		}
	}
	list = append(list, BucketInfo{Name: name, CreatedAt: time.Now().UTC()})
	if err := os.MkdirAll(filepath.Join(s.root, name, "objects"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.root, name, "meta"), 0o755); err != nil {
		return err
	}
	return s.saveBuckets(list)
}

func (s *Store) DeleteBucket(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = sanitizeBucket(name)
	list, err := s.loadBuckets()
	if err != nil {
		return err
	}
	var next []BucketInfo
	found := false
	for _, b := range list {
		if b.Name == name {
			found = true
			continue
		}
		next = append(next, b)
	}
	if !found {
		return fmt.Errorf("bucket %q not found", name)
	}
	entries, _ := os.ReadDir(filepath.Join(s.root, name, "objects"))
	if len(entries) > 0 {
		return fmt.Errorf("bucket %q is not empty", name)
	}
	_ = os.RemoveAll(filepath.Join(s.root, name))
	return s.saveBuckets(next)
}

func (s *Store) ListBuckets() ([]BucketInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.loadBuckets()
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (s *Store) bucketExists(name string) bool {
	list, err := s.loadBuckets()
	if err != nil {
		return false
	}
	for _, b := range list {
		if b.Name == name {
			return true
		}
	}
	return false
}

func (s *Store) objectPath(bucket, key string) string {
	return filepath.Join(s.root, bucket, "objects", filepath.FromSlash(key))
}
func (s *Store) metaPath(bucket, key string) string {
	return filepath.Join(s.root, bucket, "meta", filepath.FromSlash(key)+".json")
}

func (s *Store) Put(bucket, key string, r io.Reader, contentType string, meta map[string]string) (*ObjectMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket = sanitizeBucket(bucket)
	key = sanitizeKey(key)
	if !s.bucketExists(bucket) {
		return nil, fmt.Errorf("bucket %q not found", bucket)
	}
	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}
	objPath := s.objectPath(bucket, key)
	if err := os.MkdirAll(filepath.Dir(objPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(objPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	w := io.MultiWriter(f, h)
	n, err := io.Copy(w, r)
	if err != nil {
		_ = os.Remove(objPath)
		return nil, err
	}
	oetag := hex.EncodeToString(h.Sum(nil))[:16]
	om := &ObjectMeta{Key: key, Bucket: bucket, Size: n, ContentType: contentType, ETag: oetag, UploadedAt: time.Now().UTC(), Metadata: meta}
	metaPath := s.metaPath(bucket, key)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(om, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		return nil, err
	}
	return om, nil
}

func (s *Store) Get(bucket, key string) (*ObjectMeta, io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket = sanitizeBucket(bucket)
	key = sanitizeKey(key)
	meta, err := s.readMeta(bucket, key)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(s.objectPath(bucket, key))
	if err != nil {
		return nil, nil, err
	}
	return meta, f, nil
}

func (s *Store) readMeta(bucket, key string) (*ObjectMeta, error) {
	data, err := os.ReadFile(s.metaPath(bucket, key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object %q not found in bucket %q", key, bucket)
		}
		return nil, err
	}
	var om ObjectMeta
	if err := json.Unmarshal(data, &om); err != nil {
		return nil, err
	}
	return &om, nil
}

func (s *Store) Head(bucket, key string) (*ObjectMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readMeta(sanitizeBucket(bucket), sanitizeKey(key))
}

func (s *Store) Delete(bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket = sanitizeBucket(bucket)
	key = sanitizeKey(key)
	_ = os.Remove(s.objectPath(bucket, key))
	err := os.Remove(s.metaPath(bucket, key))
	if os.IsNotExist(err) {
		return fmt.Errorf("object %q not found in bucket %q", key, bucket)
	}
	return err
}

func (s *Store) ListObjects(bucket, prefix string) ([]ObjectMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket = sanitizeBucket(bucket)
	if !s.bucketExists(bucket) {
		return nil, fmt.Errorf("bucket %q not found", bucket)
	}
	metaRoot := filepath.Join(s.root, bucket, "meta")
	var out []ObjectMeta
	_ = filepath.WalkDir(metaRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var om ObjectMeta
		if err := json.Unmarshal(data, &om); err != nil {
			return nil
		}
		if prefix != "" && !strings.HasPrefix(om.Key, prefix) {
			return nil
		}
		out = append(out, om)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *Store) SignedURL(bucket, key, method string, ttl time.Duration) (string, error) {
	bucket = sanitizeBucket(bucket)
	key = sanitizeKey(key)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)
	exp := time.Now().UTC().Add(ttl).Unix()
	msg := fmt.Sprintf("%s\n%s\n%s\n%d", method, bucket, key, exp)
	sig := sign(msg)
	return fmt.Sprintf("paradox://%s/%s?method=%s&exp=%d&sig=%s", bucket, key, method, exp, sig), nil
}

func VerifySignedURL(urlStr string) (bucket, key, method string, err error) {
	if !strings.HasPrefix(urlStr, "paradox://") {
		return "", "", "", fmt.Errorf("invalid signed URL scheme")
	}
	rest := strings.TrimPrefix(urlStr, "paradox://")
	parts := strings.SplitN(rest, "?", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid signed URL")
	}
	pathPart := parts[0]
	query := parts[1]
	slash := strings.Index(pathPart, "/")
	if slash < 0 {
		return "", "", "", fmt.Errorf("invalid path")
	}
	bucket = pathPart[:slash]
	key = pathPart[slash+1:]
	var exp int64
	var sig string
	method = "GET"
	for _, kv := range strings.Split(query, "&") {
		p := strings.SplitN(kv, "=", 2)
		if len(p) != 2 {
			continue
		}
		switch p[0] {
		case "method":
			method = p[1]
		case "exp":
			fmt.Sscanf(p[1], "%d", &exp)
		case "sig":
			sig = p[1]
		}
	}
	if time.Now().Unix() > exp {
		return "", "", "", fmt.Errorf("signed URL expired")
	}
	msg := fmt.Sprintf("%s\n%s\n%s\n%d", method, bucket, key, exp)
	if !hmac.Equal([]byte(sign(msg)), []byte(sig)) {
		return "", "", "", fmt.Errorf("invalid signature")
	}
	return bucket, key, method, nil
}

func sign(msg string) string {
	key := signingSecret()
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

func signingSecret() []byte {
	if k := os.Getenv("PARADOX_STORAGE_SECRET"); k != "" {
		return []byte(k)
	}
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".paradox", "storage")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = filepath.Join(d, "storage")
	}
	path := filepath.Join(base, ".signing.key")
	if data, err := os.ReadFile(path); err == nil && len(data) >= 32 {
		return data
	}
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	_ = os.MkdirAll(base, 0o700)
	_ = os.WriteFile(path, key, 0o600)
	return key
}

func sanitizeBucket(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "\\", "/")
	for strings.Contains(key, "//") {
		key = strings.ReplaceAll(key, "//", "/")
	}
	key = strings.TrimPrefix(key, "/")
	if strings.Contains(key, "..") {
		return ""
	}
	return key
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
