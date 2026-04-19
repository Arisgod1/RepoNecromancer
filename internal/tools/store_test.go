package tools

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewTTLStore(t *testing.T) {
	// Create temp directory for test
	tmpDir := filepath.Join(os.TempDir(), "necro-test-store")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	if store == nil {
		t.Fatal("NewTTLStore returned nil")
	}
	if store.cacheDir != tmpDir {
		t.Errorf("Expected cacheDir %q, got %q", tmpDir, store.cacheDir)
	}
}

func TestNewTTLStore_EmptyDir(t *testing.T) {
	// When empty string is passed, should use temp dir
	store := NewTTLStore("")
	if store == nil {
		t.Fatal("NewTTLStore with empty string returned nil")
	}
	if !strings.Contains(store.cacheDir, "necro-cache") {
		t.Logf("Cache dir: %s", store.cacheDir)
	}
}

func TestTTLStore_SetWithTTL(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-set")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear() // Start fresh

	store.SetWithTTL("key1", "value1", 5*time.Minute)

	// Verify file was created
	entryPath := store.entryPath("key1")
	if _, err := os.Stat(entryPath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}
}

func TestTTLStore_Get_NonExistent(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-get-none")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	val, ok := store.Get("non-existent-key")
	if ok {
		t.Error("Get should return false for non-existent key")
	}
	if val != nil {
		t.Errorf("Expected nil value, got %v", val)
	}
}

func TestTTLStore_Get_Existing(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-get")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", map[string]any{"data": "test"}, 5*time.Minute)

	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Get returned false for existing key")
	}

	valMap, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("Expected map, got %T", val)
	}
	if valMap["data"] != "test" {
		t.Errorf("Expected data='test', got %v", valMap["data"])
	}
}

func TestTTLStore_Get_Expired(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-expired")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	// Set with very short TTL
	store.SetWithTTL("key1", "value1", 20*time.Millisecond)

	// Verify it exists first
	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Key should exist before expiration")
	}
	if val != "value1" {
		t.Errorf("Expected 'value1', got %v", val)
	}

	// Wait for expiration
	time.Sleep(50 * time.Millisecond)

	// Should be expired now
	val, ok = store.Get("key1")
	if ok {
		t.Error("Get should return false for expired key")
	}
}

func TestTTLStore_SetWithTTL_Overwrite(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-overwrite")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", "value1", 5*time.Minute)
	store.SetWithTTL("key1", "value2", 5*time.Minute)

	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("Get returned false for overwritten key")
	}
	if val != "value2" {
		t.Errorf("Expected 'value2', got %v", val)
	}
}

func TestTTLStore_Delete(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-delete")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", "value1", 5*time.Minute)
	store.Delete("key1")

	val, ok := store.Get("key1")
	if ok {
		t.Error("Get should return false after Delete")
	}
	if val != nil {
		t.Errorf("Expected nil, got %v", val)
	}
}

func TestTTLStore_Delete_NonExistent(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-delete-none")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	// Should not panic
	store.Delete("non-existent")
}

func TestTTLStore_Clear(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-clear")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", "value1", 5*time.Minute)
	store.SetWithTTL("key2", "value2", 5*time.Minute)
	store.SetWithTTL("key3", "value3", 5*time.Minute)

	store.Clear()

	keys := store.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after Clear, got %d", len(keys))
	}

	val, ok := store.Get("key1")
	if ok {
		t.Error("key1 should not exist after Clear")
	}
	if val != nil {
		t.Errorf("Expected nil, got %v", val)
	}
}

func TestTTLStore_Keys(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-keys")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", "value1", 5*time.Minute)
	store.SetWithTTL("key2", "value2", 5*time.Minute)

	keys := store.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestTTLStore_Keys_ExcludesExpired(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-keys-expired")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", "value1", 20*time.Millisecond)
	store.SetWithTTL("key2", "value2", 5*time.Minute)

	// Wait for key1 to expire
	time.Sleep(50 * time.Millisecond)

	keys := store.Keys()
	if len(keys) != 1 {
		t.Errorf("Expected 1 non-expired key, got %d", len(keys))
	}
}

func TestTTLStore_Stats(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-stats")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	stats := store.Stats()
	if stats.TotalKeys != 0 {
		t.Errorf("Expected 0 total keys, got %d", stats.TotalKeys)
	}

	store.SetWithTTL("key1", "value1", 5*time.Minute)
	store.SetWithTTL("key2", "value2", 5*time.Minute)

	stats = store.Stats()
	if stats.TotalKeys != 2 {
		t.Errorf("Expected 2 total keys, got %d", stats.TotalKeys)
	}
	if stats.ActiveKeys != 2 {
		t.Errorf("Expected 2 active keys, got %d", stats.ActiveKeys)
	}
}

func TestTTLStore_Stats_WithExpired(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-stats-expired")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	store.SetWithTTL("key1", "value1", 20*time.Millisecond)
	store.SetWithTTL("key2", "value2", 5*time.Minute)

	// Wait for key1 to expire
	time.Sleep(50 * time.Millisecond)

	stats := store.Stats()
	// TotalKeys counts all files including expired
	if stats.TotalKeys < 1 {
		t.Errorf("Expected at least 1 total key, got %d", stats.TotalKeys)
	}
	if stats.ActiveKeys != 1 {
		t.Errorf("Expected 1 active key, got %d", stats.ActiveKeys)
	}
}

func TestTTLStore_EntryPath(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-path")
	store := NewTTLStore(tmpDir)
	defer os.RemoveAll(tmpDir)

	path := store.entryPath("my-key")
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("Expected path to end with .json, got %s", path)
	}
	if !strings.Contains(path, tmpDir) {
		t.Errorf("Expected path to contain cache dir, got %s", path)
	}
}

func TestTTLStore_ConcurrentWrites(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-concurrent")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.SetWithTTL(string(rune('a'+i%26)), i, 5*time.Minute)
		}(i)
	}

	wg.Wait()

	// All writes should complete without error
	stats := store.Stats()
	if stats.ActiveKeys != 26 {
		t.Errorf("Expected 26 active keys, got %d", stats.ActiveKeys)
	}
}

func TestTTLStore_ConcurrentReads(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-concurrent-read")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	// Pre-populate
	for i := 0; i < 10; i++ {
		store.SetWithTTL(string(rune('a'+i)), i, 5*time.Minute)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.Get(string(rune('a' + i%10)))
		}(i)
	}

	wg.Wait()
}

func TestTTLStore_CorruptedFile(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "necro-test-corrupted")
	defer os.RemoveAll(tmpDir)

	store := NewTTLStore(tmpDir)
	store.Clear()

	// Write corrupted JSON manually
	path := store.entryPath("badkey")
	os.WriteFile(path, []byte("not valid json{"), 0644)

	val, ok := store.Get("badkey")
	if ok {
		t.Error("Get should return false for corrupted file")
	}
	if val != nil {
		t.Errorf("Expected nil value, got %v", val)
	}

	// File should have been removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Corrupted file should have been removed")
	}
}

func TestGlobalCache(t *testing.T) {
	cache := GlobalCache()
	if cache == nil {
		t.Fatal("GlobalCache returned nil")
	}

	// Should be usable
	cache.Clear()
	cache.SetWithTTL("test", "value", time.Minute)

	val, ok := cache.Get("test")
	if !ok {
		t.Error("GlobalCache should work")
	}
	if val != "value" {
		t.Errorf("Expected 'value', got %v", val)
	}

	cache.Clear()
}
