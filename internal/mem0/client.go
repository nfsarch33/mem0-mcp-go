package mem0

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Options struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

func NewClient(opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		apiKey:     opts.APIKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type MemoryRequest struct {
	Messages []Message      `json:"messages,omitempty"`
	Memory   string         `json:"memory,omitempty"`
	UserID   string         `json:"user_id,omitempty"`
	AppID    string         `json:"app_id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Infer    *bool          `json:"infer,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SearchRequest struct {
	Query   string         `json:"query"`
	UserID  string         `json:"user_id,omitempty"`
	AppID   string         `json:"app_id,omitempty"`
	Limit   int            `json:"limit,omitempty"`
	Filters map[string]any `json:"filters,omitempty"`
}

func (c *Client) Add(ctx context.Context, req MemoryRequest) (map[string]any, error) {
	return c.doJSON(ctx, http.MethodPost, "/memories", nil, req)
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (map[string]any, error) {
	return c.doJSON(ctx, http.MethodPost, "/search", nil, req)
}

func (c *Client) Get(ctx context.Context, id string) (map[string]any, error) {
	return c.doJSON(ctx, http.MethodGet, "/memories/"+url.PathEscape(id), nil, nil)
}

func (c *Client) GetAll(ctx context.Context, userID, appID string, limit int) (map[string]any, error) {
	q := url.Values{}
	if userID != "" {
		q.Set("user_id", userID)
	}
	if appID != "" {
		q.Set("app_id", appID)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	return c.doJSON(ctx, http.MethodGet, "/memories", q, nil)
}

func (c *Client) Update(ctx context.Context, id, memory string, metadata map[string]any) (map[string]any, error) {
	payload := map[string]any{"memory": memory}
	if metadata != nil {
		payload["metadata"] = metadata
	}
	return c.doJSON(ctx, http.MethodPut, "/memories/"+url.PathEscape(id), nil, payload)
}

func (c *Client) Delete(ctx context.Context, id string) (map[string]any, error) {
	return c.doJSON(ctx, http.MethodDelete, "/memories/"+url.PathEscape(id), nil, nil)
}

func (c *Client) History(ctx context.Context, id string) (map[string]any, error) {
	return c.doJSON(ctx, http.MethodGet, "/memories/"+url.PathEscape(id)+"/history", nil, nil)
}

func (c *Client) Doctor(ctx context.Context) error {
	_, err := c.doJSON(ctx, http.MethodGet, "/healthz", nil, nil)
	if err == nil {
		return nil
	}
	_, err = c.doJSON(ctx, http.MethodGet, "/docs", nil, nil)
	return err
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, payload any) (map[string]any, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mem0 request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mem0 status %d: %s", resp.StatusCode, string(raw))
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{"status": "ok"}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"status": "ok", "raw": string(raw)}, nil
	}
	return out, nil
}
