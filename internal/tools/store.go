package tools

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value      any
	expiration time.Time
}

// TTLStore is an in-memory cache with TTL support per entry.
type TTLStore struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

// NewTTLStore creates a new TTL-aware in-memory store.
func NewTTLStore() *TTLStore {
	s := &TTLStore{
		items: make(map[string]cacheEntry),
	}
	// Start background cleanup goroutine
	go s.cleanup()
	return s
}

// SetWithTTL stores a value with the given TTL duration.
func (s *TTLStore) SetWithTTL(key string, value any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiration := time.Now().Add(ttl)
	s.items[key] = cacheEntry{
		value:      value,
		expiration: expiration,
	}
}

// Get retrieves a value if it exists and hasn't expired.
func (s *TTLStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiration) {
		return nil, false
	}
	return entry.value, true
}

// Delete removes a key from the store.
func (s *TTLStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// Clear removes all entries from the store.
func (s *TTLStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]cacheEntry)
}

// Keys returns all non-expired keys.
func (s *TTLStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	out := make([]string, 0, len(s.items))
	for k, v := range s.items {
		if now.Before(v.expiration) {
			out = append(out, k)
		}
	}
	return out
}

// Stats returns statistics about the cache.
func (s *TTLStore) Stats() CacheStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var expired, active int
	for _, v := range s.items {
		if now.After(v.expiration) {
			expired++
		} else {
			active++
		}
	}
	return CacheStats{
		TotalKeys:  len(s.items),
		ActiveKeys: active,
		ExpiredKeys: expired,
	}
}

// CacheStats holds statistics about the cache state.
type CacheStats struct {
	TotalKeys  int
	ActiveKeys int
	ExpiredKeys int
}

// cleanup periodically removes expired entries.
func (s *TTLStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.removeExpired()
	}
}

func (s *TTLStore) removeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.items {
		if now.After(v.expiration) {
			delete(s.items, k)
		}
	}
}

// Package-level global cache instance.
var globalCache = NewTTLStore()

// GlobalCache returns the package-level global cache.
func GlobalCache() *TTLStore {
	return globalCache
}
