package permissions

import (
	"context"
	"testing"
)

func TestEngine_CanUseTool(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		toolName   string
		input      map[string]any
		wantBeh    string
		wantSource string
	}{
		{
			name:       "empty tool name is denied",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceSystem),
		},
		{
			name:       "unknown tool is denied",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "unknown.tool",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeBypass allows known github tool",
			cfg:        Config{Mode: ModeBypass},
			toolName:   "github.repository",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceMode),
		},
		{
			name:       "ModeDontAsk allows known github tool",
			cfg:        Config{Mode: ModeDontAsk},
			toolName:   "github.commits",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceMode),
		},
		{
			name:       "ModePlan asks for known github tool",
			cfg:        Config{Mode: ModePlan},
			toolName:   "github.issues",
			input:      nil,
			wantBeh:    string(BehaviorAsk),
			wantSource: string(SourceMode),
		},
		{
			name:       "ModeDefault github tool is allowed",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "github.repository",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeAuto github tool is allowed",
			cfg:        Config{Mode: ModeAuto},
			toolName:   "github.pull_requests",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeAcceptEdit github tool is allowed",
			cfg:        Config{Mode: ModeAcceptEdit},
			toolName:   "github.commits",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeDefault web tool without allowlist asks",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "web.fetch",
			input:      map[string]any{"url": "https://example.com"},
			wantBeh:    string(BehaviorAsk),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModePlan web tool still asks",
			cfg:        Config{Mode: ModePlan},
			toolName:   "web.fetch",
			input:      map[string]any{"url": "https://example.com"},
			wantBeh:    string(BehaviorAsk),
			wantSource: string(SourceMode),
		},
		{
			name:       "ModeBypass web tool is allowed",
			cfg:        Config{Mode: ModeBypass},
			toolName:   "web.fetch",
			input:      map[string]any{"url": "https://example.com"},
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceMode),
		},
		{
			name:       "ModeDefault write tool - github prefix matched first yields allow",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "github.write_file",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeDefault shell tool is unknown and denied",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "shell.exec",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeDefault exec tool is unknown and denied",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "tool.exec",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceRule),
		},
		{
			name:       "ModeDefault unclassified unknown tool is denied",
			cfg:        Config{Mode: ModeDefault},
			toolName:   "some.other.tool",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceRule),
		},
		{
			name:       "invalid mode denies",
			cfg:        Config{Mode: Mode("invalid")},
			toolName:   "github.repository",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceSystem),
		},
		{
			name:       "tool override allow takes precedence",
			cfg:        Config{Mode: ModeDefault, ToolAllowOverrides: map[string]Behavior{"github.write_file": BehaviorAllow}},
			toolName:   "github.write_file",
			input:      nil,
			wantBeh:    string(BehaviorAllow),
			wantSource: string(SourceOverride),
		},
		{
			name:       "tool override ask takes precedence",
			cfg:        Config{Mode: ModeBypass, ToolAllowOverrides: map[string]Behavior{"github.repository": BehaviorAsk}},
			toolName:   "github.repository",
			input:      nil,
			wantBeh:    string(BehaviorAsk),
			wantSource: string(SourceOverride),
		},
		{
			name:       "tool override deny takes precedence",
			cfg:        Config{Mode: ModeBypass, ToolAllowOverrides: map[string]Behavior{"github.commits": BehaviorDeny}},
			toolName:   "github.commits",
			input:      nil,
			wantBeh:    string(BehaviorDeny),
			wantSource: string(SourceOverride),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine(tt.cfg)
			dec, err := e.CanUseTool(context.Background(), tt.toolName, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dec.Behavior != tt.wantBeh {
				t.Errorf("behavior: got %q, want %q", dec.Behavior, tt.wantBeh)
			}
			if dec.Source != tt.wantSource {
				t.Errorf("source: got %q, want %q", dec.Source, tt.wantSource)
			}
		})
	}
}

func TestEngine_decideWeb(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		input      map[string]any
		wantBeh    string
	}{
		{
			name:       "missing url asks",
			cfg:        Config{},
			input:      map[string]any{},
			wantBeh:    string(BehaviorAsk),
		},
		{
			name:       "empty url asks",
			cfg:        Config{},
			input:      map[string]any{"url": ""},
			wantBeh:    string(BehaviorAsk),
		},
		{
			name:       "invalid url denies",
			cfg:        Config{},
			input:      map[string]any{"url": "://broken"},
			wantBeh:    string(BehaviorDeny),
		},
		{
			name:       "empty host denies",
			cfg:        Config{},
			input:      map[string]any{"url": "https://"},
			wantBeh:    string(BehaviorDeny),
		},
		{
			name:       "blocklisted domain denies",
			cfg:        Config{BlockedDomains: []string{"evil.com", "malware.net"}},
			input:      map[string]any{"url": "https://evil.com/page"},
			wantBeh:    string(BehaviorDeny),
		},
		{
			name:       "blocklisted subdomain denies",
			cfg:        Config{BlockedDomains: []string{"example.com"}},
			input:      map[string]any{"url": "https://sub.example.com/page"},
			wantBeh:    string(BehaviorDeny),
		},
		{
			name:       "allowlisted domain allows",
			cfg:        Config{AllowedDomains: []string{"github.com", "api.github.com"}},
			input:      map[string]any{"url": "https://github.com/repo"},
			wantBeh:    string(BehaviorAllow),
		},
		{
			name:       "allowlisted subdomain allows",
			cfg:        Config{AllowedDomains: []string{"github.com"}},
			input:      map[string]any{"url": "https://api.github.com/zen"},
			wantBeh:    string(BehaviorAllow),
		},
		{
			name:       "non-allowlisted domain with no allowlist asks",
			cfg:        Config{AllowedDomains: []string{}},
			input:      map[string]any{"url": "https://example.com"},
			wantBeh:    string(BehaviorAsk),
		},
		{
			name:       "domain not in allowlist asks",
			cfg:        Config{AllowedDomains: []string{"github.com"}},
			input:      map[string]any{"url": "https://example.com"},
			wantBeh:    string(BehaviorAsk),
		},
		{
			name:       "localhost denies when DenyPrivateNetworks is true",
			cfg:        Config{DenyPrivateNetworks: true},
			input:      map[string]any{"url": "http://localhost:8080"},
			wantBeh:    string(BehaviorDeny),
		},
		{
			name:       "private ip denies when DenyPrivateNetworks is true",
			cfg:        Config{DenyPrivateNetworks: true},
			input:      map[string]any{"url": "http://192.168.1.1:8080"},
			wantBeh:    string(BehaviorDeny),
		},
		{
			name:       "private hostname - DNS lookup fails so asks instead of deny",
			cfg:        Config{DenyPrivateNetworks: true},
			input:      map[string]any{"url": "http://internal.local:8080"},
			wantBeh:    string(BehaviorAsk),
		},
		{
			name:       "localhost allows when DenyPrivateNetworks is false",
			cfg:        Config{DenyPrivateNetworks: false},
			input:      map[string]any{"url": "http://localhost:8080"},
			wantBeh:    string(BehaviorAsk),
		},
		{
			name:       "domain matching is case insensitive",
			cfg:        Config{AllowedDomains: []string{"GITHUB.COM"}},
			input:      map[string]any{"url": "https://github.com/repo"},
			wantBeh:    string(BehaviorAllow),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine(tt.cfg)
			dec, _ := e.decideWeb(tt.input)
			if dec != Behavior(tt.wantBeh) {
				t.Errorf("behavior: got %q, want %q", dec, tt.wantBeh)
			}
		})
	}
}

func TestContainsDomain(t *testing.T) {
	tests := []struct {
		name     string
		list     []string
		host     string
		expected bool
	}{
		{"empty list returns false", []string{}, "github.com", false},
		{"exact match", []string{"github.com"}, "github.com", true},
		{"subdomain match", []string{"github.com"}, "api.github.com", true},
		{"subdomain of subdomain", []string{"github.com"}, "foo.bar.github.com", true},
		{"no match", []string{"github.com"}, "gitlab.com", false},
		{"empty string in list is skipped", []string{""}, "github.com", false},
		{"whitespace in list is trimmed", []string{"  github.com  "}, "github.com", true},
		{"case insensitive", []string{"GITHUB.COM"}, "github.com", true},
		{"partial suffix no match", []string{"hub.com"}, "github.com", false},
		{"suffix match only with dot", []string{"hub.com"}, "barhub.com", false},
		{"multiple domains first matches", []string{"evil.com", "github.com"}, "github.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsDomain(tt.list, tt.host)
			if got != tt.expected {
				t.Errorf("containsDomain(%v, %q) = %v, want %v", tt.list, tt.host, got, tt.expected)
			}
		})
	}
}

func TestIsPrivateOrLocalHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"localhost", "localhost", true},
		{"loopback ip", "127.0.0.1", true},
		{"another loopback", "127.0.0.2", true},
		{"10.x private", "10.0.0.1", true},
		{"10.x random", "10.255.255.255", true},
		{"172.16.x", "172.16.0.1", true},
		{"172.31.x", "172.31.255.255", true},
		{"192.168.x", "192.168.1.100", true},
		{"169.254 link-local", "169.254.0.1", true},
		{"public ip", "8.8.8.8", false},
		{"another public", "1.1.1.1", false},
		{"empty host", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrivateOrLocalHost(tt.host)
			if got != tt.expected {
				t.Errorf("isPrivateOrLocalHost(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	valid := []string{"default", "plan", "dontAsk", "bypass", "acceptEdits", "auto"}
	for _, v := range valid {
		mode, err := ParseMode(v)
		if err != nil {
			t.Errorf("ParseMode(%q) returned error: %v", v, err)
		}
		if string(mode) != v {
			t.Errorf("ParseMode(%q) = %q, want %q", v, mode, v)
		}
	}

	invalid := []string{"", "invalid", "DEFAULT", "planning"}
	for _, v := range invalid {
		_, err := ParseMode(v)
		if err == nil {
			t.Errorf("ParseMode(%q) expected error, got nil", v)
		}
	}
}
