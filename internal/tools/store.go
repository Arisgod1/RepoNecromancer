package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type cacheEntry struct {
	Value      any       `json:"value"`
	Expiration time.Time `json:"expiration"`
}

// TTLStore is a file-backed TTL cache.
type TTLStore struct {
	cacheDir string
	mu       sync.Mutex // serializes writes (reads are lock-free via file system)
}

// NewTTLStore(cacheDir string) creates a file-backed TTL cache.
// cacheDir is the directory to store cache files (e.g. ~/.cache/necro).
// If cacheDir is empty, uses os.TempDir().
func NewTTLStore(cacheDir string) *TTLStore {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "necro-cache")
	}
	os.MkdirAll(cacheDir, 0755)
	return &TTLStore{cacheDir: cacheDir}
}

func (s *TTLStore) entryPath(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(s.cacheDir, fmt.Sprintf("%x.json", h))
}

func (s *TTLStore) SetWithTTL(key string, value any, ttl time.Duration) {
	entry := cacheEntry{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := s.entryPath(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	os.WriteFile(path, data, 0644)
}

func (s *TTLStore) Get(key string) (any, bool) {
	path := s.entryPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		os.Remove(path) // corrupted
		return nil, false
	}
	if time.Now().After(entry.Expiration) {
		os.Remove(path) // expired
		return nil, false
	}
	return entry.Value, true
}

func (s *TTLStore) Delete(key string) {
	path := s.entryPath(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	os.Remove(path)
}

func (s *TTLStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, _ := os.ReadDir(s.cacheDir)
	for _, e := range entries {
		os.Remove(filepath.Join(s.cacheDir, e.Name()))
	}
}

func (s *TTLStore) Keys() []string {
	now := time.Now()
	entries, _ := os.ReadDir(s.cacheDir)
	var keys []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.cacheDir, e.Name()))
		if err != nil {
			continue
		}
		var entry cacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		if now.After(entry.Expiration) {
			continue
		}
		keys = append(keys, e.Name()) // note: this is filename, not original key - acceptable for display
	}
	return keys
}

// CacheStats holds statistics about the cache state.
type CacheStats struct {
	TotalKeys   int
	ActiveKeys  int
	ExpiredKeys int
}

func (s *TTLStore) Stats() CacheStats {
	entries, _ := os.ReadDir(s.cacheDir)
	now := time.Now()
	var active, expired int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.cacheDir, e.Name()))
		if err != nil {
			expired++
			continue
		}
		var entry cacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			expired++
			continue
		}
		if now.After(entry.Expiration) {
			expired++
		} else {
			active++
		}
	}
	return CacheStats{
		TotalKeys:   len(entries),
		ActiveKeys:  active,
		ExpiredKeys: expired,
	}
}

// Package-level global cache instance (file-backed).
var globalCache = NewTTLStore("")

// GlobalCache returns the global file-backed cache.
func GlobalCache() *TTLStore {
	return globalCache
}
