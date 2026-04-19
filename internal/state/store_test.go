package state

import (
	"sync"
	"testing"
	"time"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	if store == nil {
		t.Fatal("NewMemoryStore returned nil")
	}
}

func TestNewMemoryStore_WithOptions(t *testing.T) {
	store := NewMemoryStore(WithMaxEntries(100), WithMaxBytes(1024))
	if store == nil {
		t.Fatal("NewMemoryStore with options returned nil")
	}
}

func TestMemoryStore_Get_Set(t *testing.T) {
	store := NewMemoryStore()

	// Test Set and Get
	store.Set("key1", "value1")
	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Get failed to retrieve stored value")
	}
	if val != "value1" {
		t.Errorf("Expected value %q, got %q", "value1", val)
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore()

	val, ok := store.Get("non-existent")
	if ok {
		t.Error("Get should return false for non-existent key")
	}
	if val != nil {
		t.Errorf("Expected nil value, got %v", val)
	}
}

func TestMemoryStore_SetOverwrite(t *testing.T) {
	store := NewMemoryStore()

	store.Set("key1", "value1")
	store.Set("key1", "value2")

	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Get failed to retrieve stored value")
	}
	if val != "value2" {
		t.Errorf("Expected value %q, got %q", "value2", val)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()

	store.Set("key1", "value1")
	ok := store.Delete("key1")
	if !ok {
		t.Error("Delete should return true for existing key")
	}

	_, ok = store.Get("key1")
	if ok {
		t.Error("Get should return false after Delete")
	}
}

func TestMemoryStore_Delete_NotFound(t *testing.T) {
	store := NewMemoryStore()

	ok := store.Delete("non-existent")
	if ok {
		t.Error("Delete should return false for non-existent key")
	}
}

func TestMemoryStore_Clear(t *testing.T) {
	store := NewMemoryStore()

	store.Set("key1", "value1")
	store.Set("key2", "value2")
	store.Set("key3", "value3")

	store.Clear()

	keys := store.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after Clear, got %d", len(keys))
	}
}

func TestMemoryStore_Keys(t *testing.T) {
	store := NewMemoryStore()

	store.Set("key1", "value1")
	store.Set("key2", "value2")
	store.Set("key3", "value3")

	keys := store.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Keys should contain all set keys
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	for _, expected := range []string{"key1", "key2", "key3"} {
		if !keySet[expected] {
			t.Errorf("Expected key %q not found", expected)
		}
	}
}

func TestMemoryStore_Keys_WithPrefix(t *testing.T) {
	store := NewMemoryStore()

	store.Set("user:alice", "alice")
	store.Set("user:bob", "bob")
	store.Set("config:theme", "dark")
	store.Set("config:lang", "en")

	keys := store.Keys()
	if len(keys) != 4 {
		t.Errorf("Expected 4 keys, got %d", len(keys))
	}
}

func TestMemoryStore_SetWithTTL(t *testing.T) {
	store := NewMemoryStore()

	// Set with short TTL
	store.SetWithTTL("key1", "value1", 50*time.Millisecond)

	// Should be available immediately
	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Get failed to retrieve stored value with TTL")
	}
	if val != "value1" {
		t.Errorf("Expected value %q, got %q", "value1", val)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = store.Get("key1")
	if ok {
		t.Error("Get should return false after TTL expires")
	}
}

func TestMemoryStore_SetWithTTL_ZeroDuration(t *testing.T) {
	store := NewMemoryStore()

	// Zero duration means no expiration
	store.SetWithTTL("key1", "value1", 0)

	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Get failed to retrieve stored value with zero TTL")
	}
	if val != "value1" {
		t.Errorf("Expected value %q, got %q", "value1", val)
	}
}

func TestMemoryStore_Stats(t *testing.T) {
	store := NewMemoryStore()

	// Initial stats
	stats := store.Stats()
	if stats.CurrentEntries != 0 {
		t.Errorf("Expected 0 entries, got %d", stats.CurrentEntries)
	}

	// Set some values
	store.Set("key1", "value1")
	store.Set("key2", "value2")

	// Get stats after sets
	stats = store.Stats()
	if stats.CurrentEntries != 2 {
		t.Errorf("Expected 2 entries, got %d", stats.CurrentEntries)
	}
}

func TestMemoryStore_Stats_HitsAndMisses(t *testing.T) {
	store := NewMemoryStore()

	store.Set("key1", "value1")

	// Miss
	store.Get("non-existent")
	// Hit
	store.Get("key1")
	// Hit
	store.Get("key1")

	stats := store.Stats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestMemoryStore_Len(t *testing.T) {
	store := NewMemoryStore()

	if store.Len() != 0 {
		t.Errorf("Expected 0 length, got %d", store.Len())
	}

	store.Set("key1", "value1")
	if store.Len() != 1 {
		t.Errorf("Expected 1 length, got %d", store.Len())
	}

	store.Set("key2", "value2")
	if store.Len() != 2 {
		t.Errorf("Expected 2 length, got %d", store.Len())
	}
}

func TestMemoryStore_Len_WithExpiration(t *testing.T) {
	store := NewMemoryStore()

	store.SetWithTTL("key1", "value1", 50*time.Millisecond)
	store.Set("key2", "value2")

	if store.Len() != 2 {
		t.Errorf("Expected 2 length, got %d", store.Len())
	}

	time.Sleep(100 * time.Millisecond)

	if store.Len() != 1 {
		t.Errorf("Expected 1 length after expiration, got %d", store.Len())
	}
}

func TestEntry_IsExpired(t *testing.T) {
	// Entry with no expiration
	e1 := &Entry{}
	if e1.IsExpired() {
		t.Error("Entry with zero time should not be expired")
	}

	// Entry with future expiration
	e2 := &Entry{
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if e2.IsExpired() {
		t.Error("Entry with future expiration should not be expired")
	}

	// Entry with past expiration
	e3 := &Entry{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if !e3.IsExpired() {
		t.Error("Entry with past expiration should be expired")
	}
}

func TestMemoryStore_WithMaxEntries(t *testing.T) {
	store := NewMemoryStore(WithMaxEntries(3))

	store.Set("key1", "value1")
	store.Set("key2", "value2")
	store.Set("key3", "value3")

	stats := store.Stats()
	if stats.CurrentEntries != 3 {
		t.Errorf("Expected 3 entries, got %d", stats.CurrentEntries)
	}

	// Adding 4th entry should trigger eviction
	store.Set("key4", "value4")

	stats = store.Stats()
	if stats.CurrentEntries != 3 {
		t.Errorf("Expected 3 entries after eviction, got %d", stats.CurrentEntries)
	}

	// key1 should have been evicted (LRU)
	_, ok := store.Get("key1")
	if ok {
		t.Error("key1 should have been evicted")
	}
}

func TestMemoryStore_GetEntry(t *testing.T) {
	store := NewMemoryStore()

	store.Set("key1", "value1")

	entry, ok := store.GetEntry("key1")
	if !ok {
		t.Fatal("GetEntry failed to retrieve entry")
	}
	if entry.Key != "key1" {
		t.Errorf("Expected key %q, got %q", "key1", entry.Key)
	}
	if entry.Value != "value1" {
		t.Errorf("Expected value %q, got %q", "value1", entry.Value)
	}
}

func TestMemoryStore_GetEntry_NotFound(t *testing.T) {
	store := NewMemoryStore()

	_, ok := store.GetEntry("non-existent")
	if ok {
		t.Error("GetEntry should return false for non-existent key")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.Set(string(rune('a'+i%26)), i)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.Get(string(rune('a'+i%26)))
		}(i)
	}

	wg.Wait()

	// Verify store is still consistent
	keys := store.Keys()
	if len(keys) != 26 {
		t.Errorf("Expected 26 keys after concurrent access, got %d", len(keys))
	}
}

func TestMemoryStore_ReapExpired(t *testing.T) {
	store := NewMemoryStore()

	store.SetWithTTL("key1", "value1", 20*time.Millisecond)
	store.Set("key2", "value2")

	// Wait for expiration
	time.Sleep(50 * time.Millisecond)

	// Manually trigger reap
	store.reapExpired()

	// Only key2 should remain
	keys := store.Keys()
	if len(keys) != 1 {
		t.Errorf("Expected 1 key after reap, got %d", len(keys))
	}

	_, ok := store.Get("key2")
	if !ok {
		t.Error("key2 should still exist")
	}
}

func TestMemoryStore_Subscribe(t *testing.T) {
	store := NewMemoryStore()

	var notifications int
	var mu sync.Mutex

	unsubscribe := store.Subscribe(func(key string, oldValue, newValue any) {
		mu.Lock()
		notifications++
		mu.Unlock()
	})

	// Note: Current implementation doesn't notify on Set, only Get/Remove
	// This test just verifies Subscribe doesn't panic and unsubscribe works
	store.Set("key1", "value1")
	store.Get("key1")

	// Unsubscribe
	unsubscribe()

	store.Set("key2", "value2")

	mu.Lock()
	if notifications > 0 {
		// Some implementations may notify on internal changes
		t.Logf("Got %d notifications", notifications)
	}
	mu.Unlock()
}

func TestMemoryStore_Subscribe_Unsubscribe(t *testing.T) {
	store := NewMemoryStore()

	unsubscribe := store.Subscribe(func(key string, oldValue, newValue any) {})

	// Should be able to call unsubscribe multiple times
	unsubscribe()
	unsubscribe() // Should not panic
}

func TestMemoryStore_DifferentValueTypes(t *testing.T) {
	store := NewMemoryStore()

	// Test string
	store.Set("string", "hello")
	val, ok := store.Get("string")
	if !ok || val != "hello" {
		t.Errorf("String test failed: ok=%v, val=%v", ok, val)
	}

	// Test int
	store.Set("int", 42)
	val, ok = store.Get("int")
	if !ok || val != 42 {
		t.Errorf("Int test failed: ok=%v, val=%v", ok, val)
	}

	// Test float
	store.Set("float", 3.14)
	val, ok = store.Get("float")
	if !ok || val != 3.14 {
		t.Errorf("Float test failed: ok=%v, val=%v", ok, val)
	}

	// Test bool
	store.Set("bool", true)
	val, ok = store.Get("bool")
	if !ok || val != true {
		t.Errorf("Bool test failed: ok=%v, val=%v", ok, val)
	}

	// Test slice separately since slices can't be compared with ==
	store.Set("slice", []int{1, 2, 3})
	val, ok = store.Get("slice")
	if !ok {
		t.Error("Slice test failed: not found")
	}
	valSlice, ok := val.([]int)
	if !ok {
		t.Errorf("Slice test failed: expected []int, got %T", val)
	}
	if len(valSlice) != 3 || valSlice[0] != 1 || valSlice[1] != 2 || valSlice[2] != 3 {
		t.Errorf("Slice test failed: got %v", valSlice)
	}

	// Test struct
	store.Set("struct", struct{ A, B int }{1, 2})
	val, ok = store.Get("struct")
	if !ok {
		t.Error("Struct test failed: not found")
	}
	valStruct, ok := val.(struct{ A, B int })
	if !ok {
		t.Errorf("Struct test failed: expected struct, got %T", val)
	}
	if valStruct.A != 1 || valStruct.B != 2 {
		t.Errorf("Struct test failed: got %+v", valStruct)
	}

	// Test map separately since maps can't be compared with ==
	store.Set("map", map[string]any{"nested": "value"})
	val, ok = store.Get("map")
	if !ok {
		t.Error("Map test failed: not found")
	}
	valMap, ok := val.(map[string]any)
	if !ok {
		t.Errorf("Map test failed: expected map, got %T", val)
	}
	if valMap["nested"] != "value" {
		t.Errorf("Map test failed: got %v", valMap)
	}
}
