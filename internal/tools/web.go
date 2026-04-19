package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/repo-necromancer/necro/internal/network"
)

type webFetchTool struct {
	client *network.Client
}

func NewWebTools(client *network.Client) []Tool {
	return []Tool{
		&webFetchTool{client: client},
	}
}

func (t *webFetchTool) Name() string { return "web.fetch" }

func (t *webFetchTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	rawURL, _ := input["url"].(string)
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("url is required")
	}
	method, _ := input["method"].(string)
	if method == "" {
		method = http.MethodGet
	}
	sourceCmd, _ := input["source_command"].(string)
	sessionID, _ := input["session_id"].(string)

	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, audit, err := t.client.Do(ctx, req, sourceCmd, sessionID)
	if err != nil {
		return map[string]any{
			"error": err.Error(),
			"audit": audit,
		}, err
	}
	if resp != nil {
		defer resp.Body.Close()
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":       resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"body":         string(body),
		"audit":        audit,
	}, nil
}
