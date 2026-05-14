package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nfsarch33/mem0-mcp-go/internal/outbox"
)

func TestHealthCheck_AllHealthy(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL: upstream.URL + "/healthz",
		Timeout:     time.Second,
	})
	result := hc.Check()
	if result.Status != StatusOK {
		t.Fatalf("Status = %q, want %q", result.Status, StatusOK)
	}
	if result.Upstream != StatusOK {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusOK)
	}
	if result.OutboxPending != 0 {
		t.Fatalf("OutboxPending = %d, want 0", result.OutboxPending)
	}
}

func TestHealthCheck_UpstreamDown(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL: upstream.URL + "/healthz",
		Timeout:     time.Second,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusDown {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusDown)
	}
}

func TestHealthCheck_UpstreamUnreachable(t *testing.T) {
	t.Parallel()
	hc := NewChecker(CheckerConfig{
		UpstreamURL: "http://127.0.0.1:1/healthz",
		Timeout:     100 * time.Millisecond,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusDown {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusDown)
	}
}

func TestHealthCheck_WithOutboxPending(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	pending := &stubPendingCounter{count: 5}
	hc := NewChecker(CheckerConfig{
		UpstreamURL:    upstream.URL + "/healthz",
		Timeout:        time.Second,
		PendingCounter: pending,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q with pending items, want %q", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusOK {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusOK)
	}
	if result.OutboxPending != 5 {
		t.Fatalf("OutboxPending = %d, want 5", result.OutboxPending)
	}
}

func TestHealthCheck_WithCircuitOpen(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL:  upstream.URL + "/healthz",
		Timeout:      time.Second,
		CircuitState: func() outbox.CBState { return outbox.StateOpen },
	})
	result := hc.Check()
	if result.CircuitState != "open" {
		t.Fatalf("CircuitState = %q, want %q", result.CircuitState, "open")
	}
}

func TestHealthHandler_ReturnsJSON(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL: upstream.URL + "/healthz",
		Timeout:     time.Second,
	})
	handler := hc.Handler()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var result Result
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != StatusOK {
		t.Fatalf("Status = %q, want %q", result.Status, StatusOK)
	}
}

func TestHealthHandler_503WhenDegraded(t *testing.T) {
	t.Parallel()
	hc := NewChecker(CheckerConfig{
		UpstreamURL: "http://127.0.0.1:1/healthz",
		Timeout:     100 * time.Millisecond,
	})
	handler := hc.Handler()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("HTTP status = %d, want 503", rr.Code)
	}
}

func TestChecker_PostgresDown(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"error","detail":"pq: connection refused"}`))
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL: upstream.URL + "/healthz",
		Timeout:     time.Second,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q for postgres-down", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusDown {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusDown)
	}
}

func TestChecker_Neo4jDown(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"error","detail":"neo4j: ServiceUnavailable"}`))
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL: upstream.URL + "/healthz",
		Timeout:     time.Second,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q for neo4j-down", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusDown {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusDown)
	}
}

func TestChecker_TunnelDown(t *testing.T) {
	t.Parallel()
	// Use an unreachable address to simulate connection refused (tunnel down).
	hc := NewChecker(CheckerConfig{
		UpstreamURL: "http://127.0.0.1:1/healthz",
		Timeout:     100 * time.Millisecond,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q for tunnel-down", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusDown {
		t.Fatalf("Upstream = %q, want %q", result.Upstream, StatusDown)
	}
}

func TestChecker_PartialDegradation(t *testing.T) {
	t.Parallel()
	// Upstream returns 200 but the checker also sees pending outbox items,
	// simulating a partial degradation scenario.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"degraded","components":{"postgres":"ok","neo4j":"degraded"}}`))
	}))
	defer upstream.Close()

	pending := &stubPendingCounter{count: 3}
	hc := NewChecker(CheckerConfig{
		UpstreamURL:    upstream.URL + "/healthz",
		Timeout:        time.Second,
		PendingCounter: pending,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q for partial degradation", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusOK {
		t.Fatalf("Upstream = %q, want %q (HTTP 200 from upstream)", result.Upstream, StatusOK)
	}
	if result.OutboxPending != 3 {
		t.Fatalf("OutboxPending = %d, want 3", result.OutboxPending)
	}
}

func TestChecker_TimeoutHandling(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	hc := NewChecker(CheckerConfig{
		UpstreamURL: upstream.URL + "/healthz",
		Timeout:     50 * time.Millisecond,
	})
	result := hc.Check()
	if result.Status != StatusDegraded {
		t.Fatalf("Status = %q, want %q for timeout", result.Status, StatusDegraded)
	}
	if result.Upstream != StatusDown {
		t.Fatalf("Upstream = %q, want %q (should be down on timeout)", result.Upstream, StatusDown)
	}
}

type stubPendingCounter struct {
	count int
}

func (s *stubPendingCounter) PendingCount() int { return s.count }
