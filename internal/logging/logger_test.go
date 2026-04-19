package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Test with nil writer (should default to stderr)
	l := New(nil)
	if l == nil {
		t.Fatal("New(nil) returned nil")
	}

	// Test with valid writer
	buf := &bytes.Buffer{}
	l = New(buf)
	if l == nil {
		t.Fatal("New(buf) returned nil")
	}
	if l.w != buf {
		t.Error("Writer not set correctly")
	}
}

func TestLogger_WithSession(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	// Create a new logger with session
	sessionID := "test-session-123"
	l2 := l.WithSession(sessionID)

	// Verify the new logger has the session ID but original doesn't
	if l2.sessionID != sessionID {
		t.Errorf("Expected session ID %q, got %q", sessionID, l2.sessionID)
	}
	if l.sessionID != "" {
		t.Error("Original logger should have empty session ID")
	}
}

func TestLogger_WithCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	// Create a new logger with command
	cmd := "test-command"
	l2 := l.WithCommand(cmd)

	if l2.command != cmd {
		t.Errorf("Expected command %q, got %q", cmd, l2.command)
	}
	if l.command != "" {
		t.Error("Original logger should have empty command")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	tests := []struct {
		level    Level
		funcName string
	}{
		{LevelDebug, "Debug"},
		{LevelInfo, "Info"},
		{LevelWarn, "Warn"},
		{LevelError, "Error"},
	}

	for _, tt := range tests {
		t.Run(tt.funcName, func(t *testing.T) {
			buf := &bytes.Buffer{}
			l := New(buf)

			// Call the appropriate method
			switch tt.funcName {
			case "Debug":
				l.Debug("test message", map[string]any{"key": "value"})
			case "Info":
				l.Info("test message", map[string]any{"key": "value"})
			case "Warn":
				l.Warn("test message", map[string]any{"key": "value"})
			case "Error":
				l.Error("test message", map[string]any{"key": "value"})
			}

			// Parse the JSON output
			var entry Entry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if entry.Level != tt.level {
				t.Errorf("Expected level %q, got %q", tt.level, entry.Level)
			}

			if entry.Message != "test message" {
				t.Errorf("Expected message %q, got %q", "test message", entry.Message)
			}

			if entry.Fields["key"] != "value" {
				t.Errorf("Expected fields[key] = %q, got %q", "value", entry.Fields["key"])
			}
		})
	}
}

func TestLogger_Log_JSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	l.Info("test message", map[string]any{
		"string_field": "hello",
		"int_field":    42,
		"bool_field":   true,
		"float_field":  3.14,
	})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}

	// Verify timestamp is valid RFC3339Nano
	if _, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err != nil {
		t.Errorf("Invalid timestamp format: %v", err)
	}

	if entry.Level != LevelInfo {
		t.Errorf("Expected level %q, got %q", LevelInfo, entry.Level)
	}

	if entry.Message != "test message" {
		t.Errorf("Expected message %q, got %q", "test message", entry.Message)
	}
}

func TestLogger_Log_WithSessionID(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf).WithSession("session-abc")

	l.Info("session message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry.SessionID != "session-abc" {
		t.Errorf("Expected session ID %q, got %q", "session-abc", entry.SessionID)
	}
}

func TestLogger_Log_WithCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf).WithCommand("my-command")

	l.Info("command message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry.Command != "my-command" {
		t.Errorf("Expected command %q, got %q", "my-command", entry.Command)
	}
}

func TestLogger_Audit(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	l.Audit("user_login", map[string]any{
		"user_id": "123",
		"method":  "oauth",
	})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry.Level != LevelAudit {
		t.Errorf("Expected level %q, got %q", LevelAudit, entry.Level)
	}

	if entry.Message != "user_login" {
		t.Errorf("Expected message %q, got %q", "user_login", entry.Message)
	}

	if entry.Fields["user_id"] != "123" {
		t.Errorf("Expected fields[user_id] = %q, got %q", "123", entry.Fields["user_id"])
	}
}

func TestLogger_Audit_NilFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	// Should not panic with nil fields
	l.Audit("test_event", nil)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry.Level != LevelAudit {
		t.Errorf("Expected level %q, got %q", LevelAudit, entry.Level)
	}
}

func TestLogger_Error_WithErrorField(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	l.Error("operation failed", map[string]any{
		"error": "connection refused",
	})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry.Level != LevelError {
		t.Errorf("Expected level %q, got %q", LevelError, entry.Level)
	}

	if entry.Error != "connection refused" {
		t.Errorf("Expected error %q, got %q", "connection refused", entry.Error)
	}
}

func TestLogger_Error_WithErrorType(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	err := &testError{msg: "custom error"}
	l.Error("operation failed", map[string]any{
		"error": err,
	})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry.Error != "custom error" {
		t.Errorf("Expected error %q, got %q", "custom error", entry.Error)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestToString(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{123, "unknown"},
		{nil, ""},
		{&testError{msg: "err1"}, "err1"},
	}

	for _, tt := range tests {
		result := toString(tt.input)
		if result != tt.expected {
			t.Errorf("toString(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	buf := &bytes.Buffer{}
	l := New(buf)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l.Info("concurrent message", map[string]any{"index": i})
		}(i)
	}

	wg.Wait()

	// Verify all entries were written (check by counting newlines)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("Expected 100 log entries, got %d", len(lines))
	}
}

func TestNewSession(t *testing.T) {
	s1 := NewSession("test-command")
	s2 := NewSession("test-command")

	// Verify structure
	if s1.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if s1.Command != "test-command" {
		t.Errorf("Expected command %q, got %q", "test-command", s1.Command)
	}
	if s1.StartedAt == "" {
		t.Error("StartedAt should not be empty")
	}

	// Verify format: command-timestamp-seq
	parts := strings.Split(s1.ID, "-")
	if len(parts) < 3 {
		t.Errorf("Session ID format unexpected: %s", s1.ID)
	}

	// Verify StartedAt is valid RFC3339
	if _, err := time.Parse(time.RFC3339, s1.StartedAt); err != nil {
		t.Errorf("Invalid StartedAt format: %v", err)
	}

	// Verify sessions are unique
	if s1.ID == s2.ID {
		t.Error("Each session should have a unique ID")
	}
}

func TestSession_IDFormat(t *testing.T) {
	session := NewSession("gh")
	parts := strings.Split(session.ID, "-")

	// Expected format: {command}-{YYYYMMDDHHMMSS}-{seq}
	if len(parts) != 3 {
		t.Fatalf("Expected 3 parts in session ID, got %d: %s", len(parts), session.ID)
	}

	// First part is command
	if parts[0] != "gh" {
		t.Errorf("Expected command prefix 'gh', got %s", parts[0])
	}

	// Second part is timestamp (14 characters: YYYYMMDDHHMMSS)
	if len(parts[1]) != 14 {
		t.Errorf("Expected timestamp of 14 chars, got %d: %s", len(parts[1]), parts[1])
	}

	// Third part is sequence number (modulo 10000)
	if len(parts[2]) == 0 || len(parts[2]) > 4 {
		t.Errorf("Expected sequence number (1-4 digits), got: %s", parts[2])
	}
}

func TestSession_AtomicCounterIncrement(t *testing.T) {
	// Create multiple sessions quickly and verify IDs are unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := NewSession("counter-test")
		if ids[s.ID] {
			t.Errorf("Duplicate session ID generated: %s", s.ID)
		}
		ids[s.ID] = true
	}
}

func TestSession_Logger(t *testing.T) {
	session := NewSession("test")
	logger := session.Logger()

	if logger == nil {
		t.Fatal("Session.Logger() returned nil")
	}

	// Logger should have session ID
	if logger.sessionID != session.ID {
		t.Errorf("Expected session ID %q, got %q", session.ID, logger.sessionID)
	}

	// Logger should have command
	if logger.command != session.Command {
		t.Errorf("Expected command %q, got %q", session.Command, logger.command)
	}
}

func TestSaveAndGetSession(t *testing.T) {
	session := NewSession("save-test")
	SaveSession(session)

	retrieved, ok := GetSession(session.ID)
	if !ok {
		t.Fatal("GetSession failed to retrieve saved session")
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected ID %q, got %q", session.ID, retrieved.ID)
	}
	if retrieved.Command != session.Command {
		t.Errorf("Expected Command %q, got %q", session.Command, retrieved.Command)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	_, ok := GetSession("non-existent-id")
	if ok {
		t.Error("GetSession should return false for non-existent session")
	}
}

// Helper for tests
func TestEntry_JSONSerialization(t *testing.T) {
	entry := Entry{
		Timestamp: "2024-01-01T00:00:00Z",
		Level:     LevelInfo,
		SessionID: "session-123",
		Command:   "test",
		Message:   "hello",
		Fields:    map[string]any{"key": "value"},
		Error:     "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal Entry: %v", err)
	}

	var parsed Entry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal Entry: %v", err)
	}

	if parsed.Timestamp != entry.Timestamp {
		t.Errorf("Timestamp mismatch")
	}
	if parsed.Level != entry.Level {
		t.Errorf("Level mismatch")
	}
	if parsed.SessionID != entry.SessionID {
		t.Errorf("SessionID mismatch")
	}
}
