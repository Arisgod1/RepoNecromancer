package state

import (
	"container/list"
	"sync"
	"time"
)

// Subscriber is a callback function for store change notifications
type Subscriber func(key string, oldValue, newValue any)

// Store is the interface for the state store
type Store interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	SetWithTTL(key string, value any, ttl time.Duration)
	Delete(key string) bool
	Clear()
	Keys() []string
	Stats() StoreStats
	Subscribe(sub Subscriber) (unsubscribe func())
}

// StoreStats holds statistics about the store
type StoreStats struct {
	Hits           int64
	Misses         int64
	Evictions      int64
	Exppirations   int64
	CurrentEntries int64
	CurrentBytes   int64
}

// Entry represents a cached entry with metadata
type Entry struct {
	Key       string
	Value     any
	ExpiresAt time.Time // Zero time means no expiration
	Size      int64     // Estimated size in bytes
	element   *list.Element // Pointer to LRU list element
}

// IsExpired returns true if the entry has expired
func (e *Entry) IsExpired() bool {
	return !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt)
}

// StoreConfig holds configuration options for the store
type StoreConfig struct {
	MaxEntries int64 // Maximum number of entries (0 = unlimited)
	MaxBytes   int64 // Maximum total size in bytes (0 = unlimited)
}

// StoreOption is a function that configures the store
type StoreOption func(*StoreConfig)

// WithMaxEntries sets the maximum number of entries
func WithMaxEntries(max int64) StoreOption {
	return func(cfg *StoreConfig) {
		cfg.MaxEntries = max
	}
}

// WithMaxBytes sets the maximum total size in bytes
func WithMaxBytes(max int64) StoreOption {
	return func(cfg *StoreConfig) {
		cfg.MaxBytes = max
	}
}

// storeStats holds internal store statistics
type storeStats struct {
	Hits           int64
	Misses         int64
	Evictions      int64
	Exppirations   int64
	CurrentEntries int64
	CurrentBytes   int64
}

// MemoryStore is a thread-safe, TTL-enabled, LRU-backed in-memory store
type MemoryStore struct {
	mu          sync.RWMutex
	data        map[string]*Entry
	lru         *list.List // Most recently used at front
	subscribers map[uint64]Subscriber
	nextID      uint64
	config      StoreConfig
	stats       storeStats
}

// NewMemoryStore creates a new MemoryStore with optional configuration
func NewMemoryStore(opts ...StoreOption) *MemoryStore {
	cfg := StoreConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &MemoryStore{
		data:        make(map[string]*Entry),
		lru:         list.New(),
		subscribers: make(map[uint64]Subscriber),
		nextID:      1,
		config:      cfg,
	}
}

// Get retrieves a value by key, updating LRU and checking expiration
func (s *MemoryStore) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		s.stats.Misses++
		return nil, false
	}

	if entry.IsExpired() {
		s.removeEntry(entry)
		s.stats.Exppirations++
		s.stats.CurrentEntries--
		s.currentBytesSubtract(entry.Size)
		delete(s.data, key)
		s.stats.Misses++
		return nil, false
	}

	// Move to front of LRU (most recently used)
	s.lru.MoveToFront(entry.element)
	s.stats.Hits++
	return entry.Value, true
}

// GetEntry retrieves the full entry metadata
func (s *MemoryStore) GetEntry(key string) (*Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || entry.IsExpired() {
		return nil, false
	}
	return entry, true
}

// Set stores a value without TTL
func (s *MemoryStore) Set(key string, value any) {
	s.SetWithTTL(key, value, 0)
}

// SetWithTTL stores a value with optional TTL duration
func (s *MemoryStore) SetWithTTL(key string, value any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entrySize := s.estimateSize(value)

	// Check if key already exists
	if existing, ok := s.data[key]; ok {
		s.lru.Remove(existing.element)
		s.stats.CurrentEntries--
		s.currentBytesSubtract(existing.Size)
	}

	// Create new entry
	entry := &Entry{
		Key:   key,
		Value: value,
		Size:  entrySize,
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	// Add to LRU and map
	entry.element = s.lru.PushFront(entry)
	s.data[key] = entry
	s.stats.CurrentEntries++
	s.currentBytesAdd(entry.Size)

	// Enforce limits
	s.enforceLimitsLocked()
}

// Delete removes an entry by key
func (s *MemoryStore) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		return false
	}

	s.removeEntry(entry)
	s.stats.CurrentEntries--
	s.currentBytesSubtract(entry.Size)
	delete(s.data, key)
	return true
}

// Clear removes all entries
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*Entry)
	s.lru = list.New()
	s.stats.CurrentEntries = 0
	s.stats.CurrentBytes = 0
}

// Len returns the number of non-expired entries
func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reapExpiredLocked()
	return len(s.data)
}

// Keys returns all non-expired keys
func (s *MemoryStore) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reapExpiredLocked()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Stats returns current store statistics
func (s *MemoryStore) Stats() StoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reapExpiredLocked()
	return StoreStats{
		Hits:           s.stats.Hits,
		Misses:         s.stats.Misses,
		Evictions:      s.stats.Evictions,
		Exppirations:   s.stats.Exppirations,
		CurrentEntries: int64(len(s.data)),
		CurrentBytes:   s.calculateCurrentBytes(),
	}
}

// evict removes oldest entry from LRU (for internal use when limits exceeded)
func (s *MemoryStore) evict() {
	if s.lru.Len() == 0 {
		return
	}

	oldest := s.lru.Back()
	if oldest == nil {
		return
	}

	entry := oldest.Value.(*Entry)
	s.removeEntry(entry)
	s.stats.CurrentEntries--
	s.currentBytesSubtract(entry.Size)
	delete(s.data, entry.Key)
	s.stats.Evictions++
}

// reapExpired removes all expired entries
func (s *MemoryStore) reapExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapExpiredLocked()
}

// reapExpiredLocked removes expired entries (caller must hold lock)
func (s *MemoryStore) reapExpiredLocked() {
	now := time.Now()
	for _, entry := range s.data {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			s.removeEntry(entry)
			s.stats.CurrentEntries--
			s.currentBytesSubtract(entry.Size)
			delete(s.data, entry.Key)
			s.stats.Exppirations++
		}
	}
}

// enforceLimitsLocked enforces MaxEntries and MaxBytes limits
// Caller must hold s.mu
func (s *MemoryStore) enforceLimitsLocked() {
	// Check MaxEntries
	for s.config.MaxEntries > 0 && int64(len(s.data)) > s.config.MaxEntries {
		s.evict()
	}

	// Check MaxBytes
	for s.config.MaxBytes > 0 && s.calculateCurrentBytes() > s.config.MaxBytes {
		s.evict()
	}
}

// removeEntry removes an entry from the LRU list
func (s *MemoryStore) removeEntry(entry *Entry) {
	if entry.element != nil {
		s.lru.Remove(entry.element)
		entry.element = nil
	}
}

// currentBytesAdd adds to the current bytes counter
func (s *MemoryStore) currentBytesAdd(size int64) {
	s.stats.CurrentBytes += size
}

// currentBytesSubtract subtracts from the current bytes counter
func (s *MemoryStore) currentBytesSubtract(size int64) {
	s.stats.CurrentBytes -= size
	if s.stats.CurrentBytes < 0 {
		s.stats.CurrentBytes = 0
	}
}

// calculateCurrentBytes calculates actual current bytes from entries
func (s *MemoryStore) calculateCurrentBytes() int64 {
	var total int64
	for _, entry := range s.data {
		total += entry.Size
	}
	return total
}

// estimateSize estimates the size of a value in bytes
func (s *MemoryStore) estimateSize(value any) int64 {
	// Basic estimate for common types
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int:
		return 8
	case int32:
		return 4
	case int64:
		return 8
	case float32:
		return 4
	case float64:
		return 8
	case bool:
		return 1
	default:
		// Fallback: estimate based on interface{} overhead
		return 48
	}
}

// Subscribe adds a subscriber for store changes
func (s *MemoryStore) Subscribe(sub Subscriber) (unsubscribe func()) {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subscribers[id] = sub
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		s.mu.Unlock()
	}
}

// notifySubscribers notifies all subscribers of a change
func (s *MemoryStore) notifySubscribers(key string, oldValue, newValue any) {
	s.mu.Lock()
	snapshot := make([]Subscriber, 0, len(s.subscribers))
	for _, sub := range s.subscribers {
		snapshot = append(snapshot, sub)
	}
	s.mu.Unlock()

	for _, sub := range snapshot {
		sub(key, oldValue, newValue)
	}
}
