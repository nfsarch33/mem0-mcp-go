package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func memoriesHandler(count int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memories" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		memories := make([]map[string]any, count)
		for i := range memories {
			memories[i] = map[string]any{"id": fmt.Sprintf("m%d", i)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": memories})
	}
}

func TestQuotaCheck_BelowWarnIsOK(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(memoriesHandler(500))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: 10000,
		CritThreshold: 50000,
		Timeout:       2 * time.Second,
	})

	status, err := checker.Check(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if status.Level != LevelOK {
		t.Fatalf("level = %v, want OK", status.Level)
	}
	if status.Count != 500 {
		t.Fatalf("count = %d, want 500", status.Count)
	}
}

func TestQuotaCheck_BetweenWarnAndCriticalIsWarn(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(memoriesHandler(15000))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: 10000,
		CritThreshold: 50000,
		Timeout:       2 * time.Second,
	})

	status, err := checker.Check(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if status.Level != LevelWarn {
		t.Fatalf("level = %v, want Warn", status.Level)
	}
}

func TestQuotaCheck_AboveCriticalIsCritical(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(memoriesHandler(60000))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: 10000,
		CritThreshold: 50000,
		Timeout:       2 * time.Second,
	})

	status, err := checker.Check(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if status.Level != LevelCritical {
		t.Fatalf("level = %v, want Critical", status.Level)
	}
}

func TestQuotaCheck_APIErrorHandledGracefully(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: 10000,
		CritThreshold: 50000,
		Timeout:       2 * time.Second,
	})

	_, err := checker.Check(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error from failing API")
	}
}
