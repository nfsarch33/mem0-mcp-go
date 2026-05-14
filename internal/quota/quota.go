package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Level string

const (
	LevelOK       Level = "ok"
	LevelWarn     Level = "warn"
	LevelCritical Level = "critical"
)

type QuotaStatus struct {
	Level         Level `json:"level"`
	Count         int   `json:"count"`
	WarnThreshold int   `json:"warn_threshold"`
	CritThreshold int   `json:"crit_threshold"`
}

type Config struct {
	BaseURL       string
	APIKey        string
	WarnThreshold int
	CritThreshold int
	Timeout       time.Duration
}

func DefaultConfig() Config {
	return Config{
		BaseURL:       os.Getenv("MEM0_BASE_URL"),
		APIKey:        os.Getenv("MEM0_API_KEY"),
		WarnThreshold: envInt("MEM0_QUOTA_WARN", 10000),
		CritThreshold: envInt("MEM0_QUOTA_CRITICAL", 50000),
		Timeout:       30 * time.Second,
	}
}

type Checker struct {
	cfg    Config
	client *http.Client
}

func NewChecker(cfg Config) *Checker {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Checker{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (c *Checker) Check(ctx context.Context, userID, appID string) (QuotaStatus, error) {
	count, err := c.fetchCount(ctx, userID, appID)
	if err != nil {
		return QuotaStatus{}, fmt.Errorf("quota check: %w", err)
	}

	level := LevelOK
	if count >= c.cfg.CritThreshold {
		level = LevelCritical
	} else if count >= c.cfg.WarnThreshold {
		level = LevelWarn
	}

	return QuotaStatus{
		Level:         level,
		Count:         count,
		WarnThreshold: c.cfg.WarnThreshold,
		CritThreshold: c.cfg.CritThreshold,
	}, nil
}

func (c *Checker) fetchCount(ctx context.Context, userID, appID string) (int, error) {
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/memories"

	q := url.Values{}
	if userID != "" {
		q.Set("user_id", userID)
	}
	if appID != "" {
		q.Set("app_id", appID)
	}
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("mem0 request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("mem0 status %d: %s", resp.StatusCode, string(raw))
	}

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	results, ok := body["results"].([]any)
	if !ok {
		return 0, nil
	}
	return len(results), nil
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
