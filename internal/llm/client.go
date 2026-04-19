package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultModel   = "qwen3.6-plus"
	defaultAPIBase = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	maxRetries     = 3
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	httpClient *http.Client
	apiKey     string
	model      string
	apiBase    string
}

type chatCompletionsRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func NewClient() *Client {
	model := strings.TrimSpace(os.Getenv("DASHSCOPE_MODEL"))
	if model == "" {
		model = defaultModel
	}
	apiBase := strings.TrimSpace(os.Getenv("DASHSCOPE_API_BASE"))
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	return &Client{
		httpClient: &http.Client{Timeout: 0}, // 0 = no limit; use context deadline instead
		apiKey:     strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY")),
		model:      model,
		apiBase:    strings.TrimRight(apiBase, "/"),
	}
}

func (c *Client) Chat(systemPrompt, userPrompt string) (string, error) {
	messages := make([]Message, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, Message{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, Message{Role: "user", Content: userPrompt})
	return c.ChatWithMessages(messages)
}

func (c *Client) ChatWithMessages(messages []Message) (string, error) {
	if c == nil {
		return "", fmt.Errorf("llm client is nil")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("DASHSCOPE_API_KEY is not set")
	}
	if len(messages) == 0 {
		return "", fmt.Errorf("messages cannot be empty")
	}

	reqBody, err := json.Marshal(chatCompletionsRequest{
		Model:    c.model,
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.apiBase + "/chat/completions"
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
		if reqErr != nil {
			cancel()
			return "", fmt.Errorf("create request: %w", reqErr)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := c.httpClient.Do(req)
		cancel()
		if doErr != nil {
			lastErr = fmt.Errorf("request failed: %w", doErr)
			if attempt < maxRetries {
				time.Sleep(backoffDuration(attempt))
				continue
			}
			return "", lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response: %w", readErr)
			if attempt < maxRetries {
				time.Sleep(backoffDuration(attempt))
				continue
			}
			return "", lastErr
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("dashscope transient error: status=%d body=%s", resp.StatusCode, trimBody(body))
			if attempt < maxRetries {
				time.Sleep(backoffDuration(attempt))
				continue
			}
			return "", lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("dashscope request failed: status=%d body=%s", resp.StatusCode, trimBody(body))
		}

		var parsed chatCompletionsResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}
		if parsed.Error != nil {
			return "", fmt.Errorf("dashscope error: %s (%s)", parsed.Error.Message, parsed.Error.Code)
		}
		if len(parsed.Choices) == 0 {
			return "", fmt.Errorf("dashscope response has no choices")
		}

		content := extractText(parsed.Choices[0].Message.Content)
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("dashscope response content is empty")
		}
		return strings.TrimSpace(content), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("dashscope request failed after retries")
	}
	return "", lastErr
}

func backoffDuration(attempt int) time.Duration {
	base := 300 * time.Millisecond
	return base * time.Duration(1<<attempt)
}

func trimBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 600 {
		return s[:600] + "..."
	}
	return s
}

func extractText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, extractText(item))
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			return text
		}
		if contentArr, ok := v["content"].([]any); ok {
			return extractText(contentArr)
		}
		return ""
	default:
		return ""
	}
}
