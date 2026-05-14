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

func TestChecker_ExactWarnThreshold(t *testing.T) {
	t.Parallel()
	const warnAt = 10000

	ts := httptest.NewServer(memoriesHandler(warnAt))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: warnAt,
		CritThreshold: 50000,
		Timeout:       5 * time.Second,
	})

	status, err := checker.Check(context.Background(), "u1", "app1")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if status.Level != LevelWarn {
		t.Fatalf("level = %v, want %v (count exactly at warn threshold)", status.Level, LevelWarn)
	}
	if status.Count != warnAt {
		t.Fatalf("count = %d, want %d", status.Count, warnAt)
	}
}

func TestChecker_ExactCriticalThreshold(t *testing.T) {
	t.Parallel()
	const critAt = 50000

	ts := httptest.NewServer(memoriesHandler(critAt))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: 10000,
		CritThreshold: critAt,
		Timeout:       5 * time.Second,
	})

	status, err := checker.Check(context.Background(), "u1", "app1")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if status.Level != LevelCritical {
		t.Fatalf("level = %v, want %v (count exactly at critical threshold)", status.Level, LevelCritical)
	}
	if status.Count != critAt {
		t.Fatalf("count = %d, want %d", status.Count, critAt)
	}
}

func TestChecker_CustomThresholds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		count int
		want  Level
	}{
		{"below_warn", 499, LevelOK},
		{"at_warn", 500, LevelWarn},
		{"between", 750, LevelWarn},
		{"at_crit", 1000, LevelCritical},
		{"above_crit", 2000, LevelCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(memoriesHandler(tc.count))
			defer ts.Close()

			checker := NewChecker(Config{
				BaseURL:       ts.URL,
				APIKey:        "k",
				WarnThreshold: 500,
				CritThreshold: 1000,
				Timeout:       2 * time.Second,
			})

			status, err := checker.Check(context.Background(), "", "")
			if err != nil {
				t.Fatalf("Check error: %v", err)
			}
			if status.Level != tc.want {
				t.Fatalf("count=%d: level = %v, want %v", tc.count, status.Level, tc.want)
			}
			if status.WarnThreshold != 500 {
				t.Fatalf("warn_threshold = %d, want 500", status.WarnThreshold)
			}
			if status.CritThreshold != 1000 {
				t.Fatalf("crit_threshold = %d, want 1000", status.CritThreshold)
			}
		})
	}
}

func TestChecker_LargeCount(t *testing.T) {
	t.Parallel()
	const count = 100_000

	ts := httptest.NewServer(memoriesHandler(count))
	defer ts.Close()

	checker := NewChecker(Config{
		BaseURL:       ts.URL,
		APIKey:        "k",
		WarnThreshold: 10000,
		CritThreshold: 50000,
		Timeout:       30 * time.Second,
	})

	status, err := checker.Check(context.Background(), "u1", "")
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if status.Level != LevelCritical {
		t.Fatalf("level = %v, want %v", status.Level, LevelCritical)
	}
	if status.Count != count {
		t.Fatalf("count = %d, want %d", status.Count, count)
	}
}
