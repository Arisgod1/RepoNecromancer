package permissions

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorAsk   Behavior = "ask"
	BehaviorDeny  Behavior = "deny"
)

type Source string

const (
	SourceMode     Source = "mode"
	SourceRule     Source = "rule"
	SourceOverride Source = "override"
	SourceSystem   Source = "system"
)

type Mode string

const (
	ModeDefault    Mode = "default"
	ModePlan       Mode = "plan"
	ModeDontAsk    Mode = "dontAsk"
	ModeBypass     Mode = "bypass"
	ModeAcceptEdit Mode = "acceptEdits"
	ModeAuto       Mode = "auto"
)

type PermissionDecision struct {
	Behavior string `json:"behavior"`
	Reason   string `json:"reason"`
	Source   string `json:"source"`
}

type Config struct {
	Mode                Mode
	AllowedDomains      []string
	BlockedDomains      []string
	DenyPrivateNetworks bool
	ToolAllowOverrides  map[string]Behavior
}

type Engine struct {
	cfg Config
}

func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg}
}

func ParseMode(v string) (Mode, error) {
	switch Mode(v) {
	case ModeDefault, ModePlan, ModeDontAsk, ModeBypass, ModeAcceptEdit, ModeAuto:
		return Mode(v), nil
	default:
		return "", fmt.Errorf("invalid permission mode %q", v)
	}
}

func (e *Engine) CanUseTool(_ context.Context, toolName string, input map[string]any) (PermissionDecision, error) {
	if toolName == "" {
		return PermissionDecision{
			Behavior: string(BehaviorDeny),
			Reason:   "tool name is required",
			Source:   string(SourceSystem),
		}, nil
	}

	if override, ok := e.cfg.ToolAllowOverrides[toolName]; ok {
		return PermissionDecision{
			Behavior: string(override),
			Reason:   "tool override configured",
			Source:   string(SourceOverride),
		}, nil
	}

	if !isKnownTool(toolName) {
		return PermissionDecision{
			Behavior: string(BehaviorDeny),
			Reason:   "unknown tool",
			Source:   string(SourceRule),
		}, nil
	}

	switch e.cfg.Mode {
	case ModeBypass, ModeDontAsk:
		return PermissionDecision{
			Behavior: string(BehaviorAllow),
			Reason:   "mode allows all known tools",
			Source:   string(SourceMode),
		}, nil
	case ModePlan:
		return PermissionDecision{
			Behavior: string(BehaviorAsk),
			Reason:   "plan mode requires confirmation",
			Source:   string(SourceMode),
		}, nil
	case ModeAcceptEdit, ModeAuto, ModeDefault:
		// Continue into rule path.
	default:
		return PermissionDecision{
			Behavior: string(BehaviorDeny),
			Reason:   "invalid mode",
			Source:   string(SourceSystem),
		}, nil
	}

	switch {
	case strings.HasPrefix(toolName, "github."):
		return PermissionDecision{
			Behavior: string(BehaviorAllow),
			Reason:   "read-only GitHub API tool",
			Source:   string(SourceRule),
		}, nil
	case strings.HasPrefix(toolName, "web."):
		decision, reason := e.decideWeb(input)
		return PermissionDecision{
			Behavior: string(decision),
			Reason:   reason,
			Source:   string(SourceRule),
		}, nil
	case strings.Contains(toolName, "write"), strings.Contains(toolName, "shell"), strings.Contains(toolName, "exec"), strings.Contains(toolName, "post"):
		return PermissionDecision{
			Behavior: string(BehaviorAsk),
			Reason:   "write/exec tool requires confirmation by policy",
			Source:   string(SourceRule),
		}, nil
	default:
		return PermissionDecision{
			Behavior: string(BehaviorAsk),
			Reason:   "unclassified tool defaults to ask",
			Source:   string(SourceRule),
		}, nil
	}
}

func (e *Engine) decideWeb(input map[string]any) (Behavior, string) {
	rawURL, _ := input["url"].(string)
	if rawURL == "" {
		return BehaviorAsk, "missing url for web tool"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return BehaviorDeny, "invalid url"
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return BehaviorDeny, "url host is empty"
	}
	if containsDomain(e.cfg.BlockedDomains, host) {
		return BehaviorDeny, "domain is blocklisted"
	}
	if e.cfg.DenyPrivateNetworks && isPrivateOrLocalHost(host) {
		return BehaviorDeny, "private/internal host is denied by policy"
	}
	if len(e.cfg.AllowedDomains) == 0 {
		return BehaviorAsk, "no allowlist configured for web tools"
	}
	if containsDomain(e.cfg.AllowedDomains, host) {
		return BehaviorAllow, "domain is allowlisted"
	}
	return BehaviorAsk, "domain is not allowlisted"
}

func containsDomain(list []string, host string) bool {
	host = strings.ToLower(host)
	for _, d := range list {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func isPrivateOrLocalHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		addrs, err := net.LookupIP(host)
		if err != nil || len(addrs) == 0 {
			return false
		}
		for _, resolved := range addrs {
			if isPrivateIP(resolved) {
				return true
			}
		}
		return false
	}
	return isPrivateIP(ip)
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateRanges {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func isKnownTool(name string) bool {
	return strings.HasPrefix(name, "github.") || strings.HasPrefix(name, "web.")
}
