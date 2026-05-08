package mem0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := NewClient(Options{BaseURL: ts.URL, APIKey: "k", Timeout: 2 * time.Second})
	return ts, c
}

func okHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}
}

func countingHandler(counter *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		counter.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}
}

func errorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}
}

func TestDualWriter_AddFansOutToShadow(t *testing.T) {
	t.Parallel()
	var primaryHits, shadowHits atomic.Int64

	_, primary := newTestServer(t, countingHandler(&primaryHits))
	_, shadow := newTestServer(t, countingHandler(&shadowHits))

	dw := NewDualWriter(primary, shadow, nil, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	out, err := dw.Add(t.Context(), MemoryRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		UserID:   "u1",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("primary status = %v", out["status"])
	}
	if primaryHits.Load() != 1 {
		t.Fatalf("primary hits = %d, want 1", primaryHits.Load())
	}

	waitFor(t, &shadowHits, 1, 2*time.Second)
}

func TestDualWriter_AddFansOutToBackup(t *testing.T) {
	t.Parallel()
	var primaryHits, shadowHits, backupHits atomic.Int64

	_, primary := newTestServer(t, countingHandler(&primaryHits))
	_, shadow := newTestServer(t, countingHandler(&shadowHits))
	_, backup := newTestServer(t, countingHandler(&backupHits))

	dw := NewDualWriter(primary, shadow, backup, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	_, err := dw.Add(t.Context(), MemoryRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	waitFor(t, &shadowHits, 1, 2*time.Second)
	waitFor(t, &backupHits, 1, 2*time.Second)
}

func TestDualWriter_UpdateFansOut(t *testing.T) {
	t.Parallel()
	var shadowHits atomic.Int64

	_, primary := newTestServer(t, okHandler())
	_, shadow := newTestServer(t, countingHandler(&shadowHits))

	dw := NewDualWriter(primary, shadow, nil, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	_, err := dw.Update(t.Context(), "m1", "updated text", nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	waitFor(t, &shadowHits, 1, 2*time.Second)
}

func TestDualWriter_DeleteFansOut(t *testing.T) {
	t.Parallel()
	var shadowHits atomic.Int64

	_, primary := newTestServer(t, okHandler())
	_, shadow := newTestServer(t, countingHandler(&shadowHits))

	dw := NewDualWriter(primary, shadow, nil, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	_, err := dw.Delete(t.Context(), "m1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	waitFor(t, &shadowHits, 1, 2*time.Second)
}

func TestDualWriter_ShadowErrorNeverBlocksPrimary(t *testing.T) {
	t.Parallel()
	_, primary := newTestServer(t, okHandler())
	_, shadow := newTestServer(t, errorHandler())

	dw := NewDualWriter(primary, shadow, nil, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	out, err := dw.Add(t.Context(), MemoryRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Add should succeed even if shadow fails: %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("status = %v", out["status"])
	}

	// Wait for async goroutine to finish and check stats.
	time.Sleep(500 * time.Millisecond)
	stats := dw.Stats()
	if stats.ShadowErrors != 1 {
		t.Fatalf("ShadowErrors = %d, want 1", stats.ShadowErrors)
	}
}

func TestDualWriter_ReadRoutesToOSS(t *testing.T) {
	t.Parallel()
	var primaryHits, shadowHits atomic.Int64

	_, primary := newTestServer(t, countingHandler(&primaryHits))
	_, shadow := newTestServer(t, countingHandler(&shadowHits))

	dw := NewDualWriter(primary, shadow, nil, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	_, _ = dw.Search(t.Context(), SearchRequest{Query: "test"})
	if primaryHits.Load() != 1 {
		t.Fatalf("oss read: primary hits = %d, want 1", primaryHits.Load())
	}
	if shadowHits.Load() != 0 {
		t.Fatalf("oss read: shadow hits = %d, want 0", shadowHits.Load())
	}
}

func TestDualWriter_ReadRoutesToCloud(t *testing.T) {
	t.Parallel()
	var primaryHits, shadowHits atomic.Int64

	_, primary := newTestServer(t, countingHandler(&primaryHits))
	_, shadow := newTestServer(t, countingHandler(&shadowHits))

	dw := NewDualWriter(primary, shadow, nil, DualWriterConfig{
		ReadSource:    "cloud",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	_, _ = dw.Search(t.Context(), SearchRequest{Query: "test"})
	if shadowHits.Load() != 1 {
		t.Fatalf("cloud read: shadow hits = %d, want 1", shadowHits.Load())
	}
	if primaryHits.Load() != 0 {
		t.Fatalf("cloud read: primary hits = %d, want 0", primaryHits.Load())
	}
}

func TestDualWriter_NoShadowNoBackup(t *testing.T) {
	t.Parallel()
	var primaryHits atomic.Int64

	_, primary := newTestServer(t, countingHandler(&primaryHits))

	dw := NewDualWriter(primary, nil, nil, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	_, err := dw.Add(t.Context(), MemoryRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if primaryHits.Load() != 1 {
		t.Fatalf("primary hits = %d, want 1", primaryHits.Load())
	}

	stats := dw.Stats()
	if stats.ShadowWrites != 0 || stats.BackupWrites != 0 {
		t.Fatalf("should have no shadow/backup writes")
	}
}

func TestDualWriter_StatsCountCorrectly(t *testing.T) {
	t.Parallel()
	var shadowHits, backupHits atomic.Int64

	_, primary := newTestServer(t, okHandler())
	_, shadow := newTestServer(t, countingHandler(&shadowHits))
	_, backup := newTestServer(t, countingHandler(&backupHits))

	dw := NewDualWriter(primary, shadow, backup, DualWriterConfig{
		ReadSource:    "oss",
		ShadowTimeout: 2 * time.Second,
	}, nil)

	for range 3 {
		_, _ = dw.Add(t.Context(), MemoryRequest{
			Messages: []Message{{Role: "user", Content: "hello"}},
		})
	}

	waitFor(t, &shadowHits, 3, 2*time.Second)
	waitFor(t, &backupHits, 3, 2*time.Second)

	stats := dw.Stats()
	if stats.ShadowWrites != 3 {
		t.Fatalf("ShadowWrites = %d, want 3", stats.ShadowWrites)
	}
	if stats.BackupWrites != 3 {
		t.Fatalf("BackupWrites = %d, want 3", stats.BackupWrites)
	}
}

func TestDualWriter_DoctorCallsPrimary(t *testing.T) {
	t.Parallel()
	var primaryHits atomic.Int64

	handler := http.NewServeMux()
	handler.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		primaryHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	primary := NewClient(Options{BaseURL: ts.URL, APIKey: "k", Timeout: 2 * time.Second})

	dw := NewDualWriter(primary, nil, nil, DualWriterConfig{ReadSource: "oss"}, nil)
	if err := dw.Doctor(t.Context()); err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if primaryHits.Load() != 1 {
		t.Fatalf("primary doctor hits = %d, want 1", primaryHits.Load())
	}
}

// waitFor polls an atomic counter until it reaches the target or times out.
func waitFor(t *testing.T, counter *atomic.Int64, target int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if counter.Load() >= target {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("counter = %d, want >= %d after %v", counter.Load(), target, timeout)
}
