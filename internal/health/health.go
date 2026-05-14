package health

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nfsarch33/mem0-mcp-go/internal/outbox"
)

const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
	StatusDown     = "down"
)

// PendingCounter reports the number of entries waiting in the outbox.
type PendingCounter interface {
	PendingCount() int
}

// Result is the JSON payload returned by the /healthz endpoint.
type Result struct {
	Status        string `json:"status"`
	Upstream      string `json:"upstream"`
	OutboxPending int    `json:"outbox_pending"`
	CircuitState  string `json:"circuit_state,omitempty"`
}

// CheckerConfig holds the configuration for the health checker.
type CheckerConfig struct {
	UpstreamURL    string
	Timeout        time.Duration
	PendingCounter PendingCounter
	CircuitState   func() outbox.CBState
}

// Checker pings the upstream mem0 API and reports combined health status.
type Checker struct {
	upstreamURL    string
	httpClient     *http.Client
	pendingCounter PendingCounter
	circuitState   func() outbox.CBState
}

// NewChecker creates a health checker with the given config.
func NewChecker(cfg CheckerConfig) *Checker {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Checker{
		upstreamURL:    cfg.UpstreamURL,
		httpClient:     &http.Client{Timeout: timeout},
		pendingCounter: cfg.PendingCounter,
		circuitState:   cfg.CircuitState,
	}
}

// Check performs the health check and returns a Result.
func (c *Checker) Check() Result {
	r := Result{
		Status:   StatusOK,
		Upstream: StatusOK,
	}

	if !c.pingUpstream() {
		r.Upstream = StatusDown
		r.Status = StatusDegraded
	}

	if c.pendingCounter != nil {
		r.OutboxPending = c.pendingCounter.PendingCount()
		if r.OutboxPending > 0 {
			r.Status = StatusDegraded
		}
	}

	if c.circuitState != nil {
		r.CircuitState = c.circuitState().String()
	}

	return r
}

// Handler returns an http.HandlerFunc that serves the health check as JSON.
func (c *Checker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		result := c.Check()
		w.Header().Set("Content-Type", "application/json")
		if result.Status != StatusOK {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}

func (c *Checker) pingUpstream() bool {
	if c.upstreamURL == "" {
		return false
	}
	resp, err := c.httpClient.Get(c.upstreamURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
