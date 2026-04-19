package network

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct {
	TimeoutMS           int
	RetryMax            int
	BackoffBaseMS       int
	DenyPrivateNetworks bool
}

type AuditEntry struct {
	Timestamp  string `json:"timestamp"`
	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	LatencyMS  int64  `json:"latency_ms"`
	ContentSHA string `json:"content_sha,omitempty"`
	SourceCmd  string `json:"source_command"`
	SessionID  string `json:"session_id"`
	Error      string `json:"error,omitempty"`
}

type ToolError struct {
	Kind      string `json:"kind"`
	Retryable bool   `json:"retryable"`
	Hint      string `json:"hint,omitempty"`
	Err       error  `json:"-"`
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("%s (retryable=%t): %v", e.Kind, e.Retryable, e.Err)
}

func (e *ToolError) Unwrap() error { return e.Err }

type Client struct {
	cfg    Config
	client *http.Client
	mu     sync.Mutex
	audit  []AuditEntry
}

func NewClient(cfg Config) *Client {
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = 12000
	}
	if cfg.RetryMax < 0 {
		cfg.RetryMax = 0
	}
	if cfg.BackoffBaseMS <= 0 {
		cfg.BackoffBaseMS = 300
	}
	return &Client{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

func (c *Client) AuditTrail() []AuditEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]AuditEntry, len(c.audit))
	copy(out, c.audit)
	return out
}

func (c *Client) Do(ctx context.Context, req *http.Request, sourceCommand, sessionID string) (*http.Response, AuditEntry, error) {
	if req == nil {
		return nil, AuditEntry{}, &ToolError{Kind: "unknown", Retryable: false, Hint: "request is required", Err: errors.New("nil request")}
	}
	if err := c.enforceNetworkPolicy(req.URL); err != nil {
		entry := AuditEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			RequestID: requestID(),
			Method:    req.Method,
			URL:       req.URL.String(),
			SourceCmd: sourceCommand,
			SessionID: sessionID,
			Error:     err.Error(),
		}
		c.appendAudit(entry)
		return nil, entry, err
	}

	if req.Method == "" {
		req.Method = http.MethodGet
	}
	reqID := requestID()
	req.Header.Set("X-Necro-Request-ID", reqID)

	var lastErr error
	maxAttempts := c.cfg.RetryMax + 1
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			req = req.Clone(ctx)
		}
		start := time.Now()
		res, err := c.client.Do(req.WithContext(ctx))
		latency := time.Since(start)

		entry := AuditEntry{
			Timestamp: start.UTC().Format(time.RFC3339),
			RequestID: reqID,
			Method:    req.Method,
			URL:       req.URL.String(),
			SourceCmd: sourceCommand,
			SessionID: sessionID,
			LatencyMS: latency.Milliseconds(),
		}

		if err != nil {
			terr := classifyError(err)
			entry.Error = terr.Error()
			c.appendAudit(entry)
			lastErr = terr
			if !terr.Retryable || attempt == maxAttempts {
				return nil, entry, terr
			}
			if waitErr := sleepWithBackoff(ctx, c.cfg.BackoffBaseMS, attempt); waitErr != nil {
				return nil, entry, &ToolError{Kind: "unknown", Retryable: false, Hint: "request cancelled during backoff", Err: waitErr}
			}
			continue
		}

		entry.Status = res.StatusCode
		if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= http.StatusInternalServerError {
			c.appendAudit(entry)
			lastErr = &ToolError{
				Kind:      "rate_limit",
				Retryable: res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500,
				Hint:      "retry with backoff or reduce request rate",
				Err:       fmt.Errorf("server returned status %d", res.StatusCode),
			}
			if attempt < maxAttempts {
				_ = res.Body.Close()
				if waitErr := sleepWithBackoff(ctx, c.cfg.BackoffBaseMS, attempt); waitErr != nil {
					return nil, entry, &ToolError{Kind: "unknown", Retryable: false, Hint: "request cancelled during backoff", Err: waitErr}
				}
				continue
			}
			return res, entry, lastErr
		}

		bodyBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			_ = res.Body.Close()
			entry.Error = readErr.Error()
			c.appendAudit(entry)
			return nil, entry, &ToolError{Kind: "unknown", Retryable: false, Hint: "failed to read response body", Err: readErr}
		}
		_ = res.Body.Close()
		sum := sha256.Sum256(bodyBytes)
		entry.ContentSHA = hex.EncodeToString(sum[:])
		c.appendAudit(entry)
		res.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		return res, entry, nil
	}

	return nil, AuditEntry{}, lastErr
}

func (c *Client) Fetch(ctx context.Context, rawURL, sourceCommand, sessionID string) ([]byte, AuditEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, AuditEntry{}, &ToolError{Kind: "unknown", Retryable: false, Hint: "invalid fetch url", Err: err}
	}
	resp, audit, err := c.Do(ctx, req, sourceCommand, sessionID)
	if err != nil {
		return nil, audit, err
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, audit, &ToolError{Kind: "unknown", Retryable: false, Hint: "failed reading response body", Err: readErr}
	}
	return body, audit, nil
}

func (c *Client) enforceNetworkPolicy(u *url.URL) error {
	if u == nil {
		return &ToolError{Kind: "unknown", Retryable: false, Hint: "request URL is missing", Err: errors.New("nil url")}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return &ToolError{Kind: "unknown", Retryable: false, Hint: "only http/https are supported", Err: fmt.Errorf("unsupported scheme %q", u.Scheme)}
	}
	if !c.cfg.DenyPrivateNetworks {
		return nil
	}
	host := u.Hostname()
	if host == "" {
		return &ToolError{Kind: "dns", Retryable: false, Hint: "host is required", Err: errors.New("empty host")}
	}
	if host == "localhost" {
		return &ToolError{Kind: "dns", Retryable: false, Hint: "localhost access is blocked by policy", Err: errors.New("blocked localhost")}
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return &ToolError{Kind: "dns", Retryable: false, Hint: "private network access is blocked by policy", Err: errors.New("blocked private IP")}
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return &ToolError{Kind: "dns", Retryable: true, Hint: "failed to resolve host", Err: err}
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return &ToolError{Kind: "dns", Retryable: false, Hint: "private network access is blocked by policy", Err: errors.New("blocked private network target")}
		}
	}
	return nil
}

func (c *Client) appendAudit(entry AuditEntry) {
	c.mu.Lock()
	c.audit = append(c.audit, entry)
	c.mu.Unlock()
}

func classifyError(err error) *ToolError {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(msg, "timeout"):
		return &ToolError{Kind: "timeout", Retryable: true, Hint: "increase timeout or retry later", Err: err}
	case strings.Contains(msg, "tls"):
		return &ToolError{Kind: "tls", Retryable: false, Hint: "inspect TLS configuration", Err: err}
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "dial tcp"), strings.Contains(msg, "lookup"):
		return &ToolError{Kind: "dns", Retryable: true, Hint: "verify DNS/network connectivity", Err: err}
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "unauthorized"):
		return &ToolError{Kind: "auth", Retryable: false, Hint: "verify credentials/token", Err: err}
	default:
		return &ToolError{Kind: "unknown", Retryable: true, Hint: "retry with backoff", Err: err}
	}
}

func requestID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, 16)
	for i := range buf {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		buf[i] = chars[n.Int64()]
	}
	return "req_" + string(buf)
}

func sleepWithBackoff(ctx context.Context, baseMS, attempt int) error {
	exp := math.Pow(2, float64(attempt-1))
	backoff := float64(baseMS) * exp
	maxJitter := int64(baseMS / 2)
	if maxJitter < 1 {
		maxJitter = 1
	}
	jitterN, _ := rand.Int(rand.Reader, big.NewInt(maxJitter))
	delay := time.Duration(backoff+float64(jitterN.Int64())) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a AuditEntry) MarshalJSON() ([]byte, error) {
	type alias AuditEntry
	return json.Marshal(alias(a))
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
