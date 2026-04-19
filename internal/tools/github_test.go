package tools

import (
	"testing"
	"time"

	"github.com/google/go-github/v45/github"
)

func TestAsInt(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		fallback int
		expected int
	}{
		{"int value", 42, 0, 42},
		{"int64 value", int64(100), 0, 100},
		{"float64 value", float64(99.5), 0, 99},
		{"string value", "not a number", 10, 10},
		{"nil value", nil, 5, 5},
		{"nested value", []any{1, 2}, 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := asInt(tt.input, tt.fallback)
			if result != tt.expected {
				t.Errorf("asInt(%v, %d) = %d, want %d", tt.input, tt.fallback, result, tt.expected)
			}
		})
	}
}

func TestAsStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected []string
	}{
		{
			name:     "string slice",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "any slice with strings",
			input:    []any{"x", "y", "z"},
			expected: []string{"x", "y", "z"},
		},
		{
			name:     "any slice with mixed types",
			input:    []any{"keep", 123, "", "valid"},
			expected: []string{"keep", "valid"},
		},
		{
			name:     "any slice with whitespace strings",
			input:    []any{"  ", "trim", "  "},
			expected: []string{"trim"},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string input (not a slice)",
			input:    "not a slice",
			expected: nil,
		},
		{
			name:     "empty any slice",
			input:    []any{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := asStringSlice(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("asStringSlice(%v) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("asStringSlice(%v)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestParseOptionalTime(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    any
		expected time.Time
	}{
		{
			name:     "empty string",
			input:    "",
			expected: time.Time{},
		},
		{
			name:     "whitespace string",
			input:    "   ",
			expected: time.Time{},
		},
		{
			name:     "RFC3339 format",
			input:    "2024-01-15T10:30:00Z",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "RFC3339 with timezone",
			input:    "2024-01-15T10:30:00+05:00",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("+05:00", 5*60*60)),
		},
		{
			name:     "date only format",
			input:    "2024-01-15",
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "invalid format",
			input:    "not a date",
			expected: time.Time{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: time.Time{},
		},
		{
			name:     "non-string input",
			input:    12345,
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOptionalTime(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("parseOptionalTime(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}

	// Capture baseTime for unused variable warning
	_ = baseTime
}

func TestFormatOptionalTime(t *testing.T) {
	tests := []struct {
		name     string
		input    github.Timestamp
		fallback string
		expected string
	}{
		{
			name:     "zero time",
			input:    github.Timestamp{Time: time.Time{}},
			fallback: "never",
			expected: "never",
		},
		{
			name:     "valid time",
			input:    github.Timestamp{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
			fallback: "never",
			expected: "2024-01-15T10:30:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatOptionalTime(tt.input, tt.fallback)
			if result != tt.expected {
				t.Errorf("formatOptionalTime(%v, %q) = %q, want %q", tt.input, tt.fallback, result, tt.expected)
			}
		})
	}
}

func TestInactivityYears(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		ts       github.Timestamp
		minValue float64
		maxValue float64
	}{
		{
			name:     "zero timestamp",
			ts:       github.Timestamp{Time: time.Time{}},
			minValue: 0,
			maxValue: 0,
		},
		{
			name:     "recent timestamp",
			ts:       github.Timestamp{Time: now.Add(-24 * time.Hour)},
			minValue: 0.002,  // ~1 day / 365 days
			maxValue: 0.003,
		},
		{
			name:     "one year ago",
			ts:       github.Timestamp{Time: now.Add(-365 * 24 * time.Hour)},
			minValue: 0.9,
			maxValue: 1.1,
		},
		{
			name:     "five years ago",
			ts:       github.Timestamp{Time: now.Add(-5 * 365 * 24 * time.Hour)},
			minValue: 4.9,
			maxValue: 5.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inactivityYears(tt.ts)
			if result < tt.minValue || result > tt.maxValue {
				t.Errorf("inactivityYears(%v) = %v, want between %v and %v", tt.ts, result, tt.minValue, tt.maxValue)
			}
		})
	}
}

func TestTTLConstants(t *testing.T) {
	// Verify TTL constants are defined and positive
	if TTLNormal <= 0 {
		t.Error("TTLNormal should be positive")
	}
	if TTLGitHubHIT <= 0 {
		t.Error("TTLGitHubHIT should be positive")
	}
	if TTL404Dead <= 0 {
		t.Error("TTL404Dead should be positive")
	}
	if TTLError <= 0 {
		t.Error("TTLError should be positive")
	}

	// Verify reasonable values (in range)
	if TTLNormal < time.Minute || TTLNormal > time.Hour {
		t.Errorf("TTLNormal = %v, expected between 1m and 1h", TTLNormal)
	}
	if TTLGitHubHIT < time.Minute || TTLGitHubHIT > 10*time.Minute {
		t.Errorf("TTLGitHubHIT = %v, expected between 1m and 10m", TTLGitHubHIT)
	}
	if TTL404Dead < 30*time.Minute || TTL404Dead > 24*time.Hour {
		t.Errorf("TTL404Dead = %v, expected between 30m and 24h", TTL404Dead)
	}
}
