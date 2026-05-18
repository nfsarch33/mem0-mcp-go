package mem0

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/nfsarch33/mem0-mcp-go/internal/cache"
	"github.com/nfsarch33/mem0-mcp-go/internal/metrics"
)

const inferRetryBackoff = 60 * time.Second

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	Metrics    *metrics.Collector
	cache      *cache.Cache
}

type Options struct {
	BaseURL        string
	APIKey         string
	Timeout        time.Duration
	CacheMaxItems  int
	CacheTTL       time.Duration
}

func NewClient(opts Options) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		apiKey:     opts.APIKey,
		httpClient: &http.Client{Timeout: timeout},
		Metrics:    metrics.NewCollector(),
		cache: cache.New(cache.Options{
			MaxEntries: opts.CacheMaxItems,
			TTL:        opts.CacheTTL,
		}),
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
	Query   string         `json:"-"`
	UserID  string         `json:"-"`
	AppID   string         `json:"-"`
	Limit   int            `json:"-"`
	Filters map[string]any `json:"-"`
}

// wirePayload builds the JSON body for /search. Mem0 OSS rejects
// top-level user_id/app_id; they must be inside the filters dict.
func (sr SearchRequest) wirePayload() map[string]any {
	payload := map[string]any{"query": sr.Query}
	if sr.Limit > 0 {
		payload["limit"] = sr.Limit
	}

	filters := make(map[string]any)
	for k, v := range sr.Filters {
		filters[k] = v
	}
	if sr.UserID != "" {
		filters["user_id"] = sr.UserID
	}
	if sr.AppID != "" {
		filters["app_id"] = sr.AppID
	}
	if len(filters) > 0 {
		payload["filters"] = filters
	}
	return payload
}

func (c *Client) Add(ctx context.Context, req MemoryRequest) (map[string]any, error) {
	return c.timed("add", func() (map[string]any, error) {
		result, err := c.doJSON(ctx, http.MethodPost, "/memories", nil, req)
		if err != nil && req.Infer != nil && *req.Infer {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("add with infer: %w", ctx.Err())
			case <-time.After(inferRetryBackoff):
			}
			result, err = c.doJSON(ctx, http.MethodPost, "/memories", nil, req)
			if err != nil {
				return nil, fmt.Errorf("add with infer (retry): %w", err)
			}
		}
		return result, err
	})
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (map[string]any, error) {
	cacheKey := searchCacheKey(req)

	if raw, ok := c.cache.Get(cacheKey); ok {
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err == nil {
			return out, nil
		}
	}

	result, err := c.timed("search", func() (map[string]any, error) {
		return c.doJSON(ctx, http.MethodPost, "/search", nil, req.wirePayload())
	})
	if err != nil {
		return nil, err
	}

	if raw, marshalErr := json.Marshal(result); marshalErr == nil {
		c.cache.Set(cacheKey, raw)
	}
	return result, nil
}

// SearchCache exposes cache stats for observability.
func (c *Client) SearchCache() cache.CacheStats {
	return c.cache.Stats()
}

func searchCacheKey(req SearchRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Query))
	h.Write([]byte{0})
	h.Write([]byte(req.UserID))
	h.Write([]byte{0})
	h.Write([]byte(req.AppID))
	h.Write([]byte{0})
	if len(req.Filters) > 0 {
		keys := make([]string, 0, len(req.Filters))
		for k := range req.Filters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte{0})
			h.Write([]byte(fmt.Sprintf("%v", req.Filters[k])))
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) Get(ctx context.Context, id string) (map[string]any, error) {
	return c.timed("get", func() (map[string]any, error) {
		return c.doJSON(ctx, http.MethodGet, "/memories/"+url.PathEscape(id), nil, nil)
	})
}

func (c *Client) GetAll(ctx context.Context, userID, appID string, limit int) (map[string]any, error) {
	return c.timed("get_all", func() (map[string]any, error) {
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
	})
}

func (c *Client) Update(ctx context.Context, id, memory string, metadata map[string]any) (map[string]any, error) {
	return c.timed("update", func() (map[string]any, error) {
		payload := map[string]any{"text": memory}
		if metadata != nil {
			payload["metadata"] = metadata
		}
		return c.doJSON(ctx, http.MethodPut, "/memories/"+url.PathEscape(id), nil, payload)
	})
}

func (c *Client) Delete(ctx context.Context, id string) (map[string]any, error) {
	return c.timed("delete", func() (map[string]any, error) {
		return c.doJSON(ctx, http.MethodDelete, "/memories/"+url.PathEscape(id), nil, nil)
	})
}

func (c *Client) History(ctx context.Context, id string) (map[string]any, error) {
	return c.timed("history", func() (map[string]any, error) {
		return c.doJSON(ctx, http.MethodGet, "/memories/"+url.PathEscape(id)+"/history", nil, nil)
	})
}

func (c *Client) timed(op string, fn func() (map[string]any, error)) (map[string]any, error) {
	start := time.Now()
	result, err := fn()
	c.Metrics.Record(op, time.Since(start), err)
	return result, err
}

func (c *Client) Doctor(ctx context.Context) error {
	_, err := c.doJSON(ctx, http.MethodGet, "/healthz", nil, nil)
	if err == nil {
		return nil
	}
	_, err = c.doJSON(ctx, http.MethodGet, "/docs", nil, nil)
	return err
}

// ListEntities returns user/agent/run entities known to Mem0 OSS.
// Mem0 OSS exposes /entities; older managed Mem0 had no equivalent. Used by
// the CLI surface to enumerate scopes for handoff/preference routing.
func (c *Client) ListEntities(ctx context.Context) (map[string]any, error) {
	return c.timed("list_entities", func() (map[string]any, error) {
		raw, err := c.doRaw(ctx, http.MethodGet, "/entities", nil, nil)
		if err != nil {
			return nil, err
		}
		var arr []any
		if jerr := json.Unmarshal(raw, &arr); jerr == nil {
			return map[string]any{"entities": arr}, nil
		}
		var obj map[string]any
		if jerr := json.Unmarshal(raw, &obj); jerr == nil {
			return obj, nil
		}
		return map[string]any{"raw": string(raw)}, nil
	})
}

// doRaw is the byte-level sibling of doJSON for endpoints that return JSON
// arrays rather than objects.
func (c *Client) doRaw(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(b)
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
	return raw, nil
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
