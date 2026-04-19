package state

import "sync"

type Subscriber func(key string, oldValue, newValue any)

type Store interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	Subscribe(sub Subscriber) (unsubscribe func())
}

type MemoryStore struct {
	mu          sync.RWMutex
	data        map[string]any
	subscribers map[uint64]Subscriber
	nextID      uint64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data:        make(map[string]any),
		subscribers: make(map[uint64]Subscriber),
		nextID:      1,
	}
}

func (s *MemoryStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *MemoryStore) Set(key string, value any) {
	s.mu.Lock()
	old := s.data[key]
	s.data[key] = value
	snapshot := make([]Subscriber, 0, len(s.subscribers))
	for _, sub := range s.subscribers {
		snapshot = append(snapshot, sub)
	}
	s.mu.Unlock()

	for _, sub := range snapshot {
		sub(key, old, value)
	}
}

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
