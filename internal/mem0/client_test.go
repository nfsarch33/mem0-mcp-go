package mem0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAddUsesOSSHeadersAndPath(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memories" {
			t.Fatalf("path = %q, want /memories", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Fatal("missing X-API-Key")
		}
		var req MemoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.UserID != "nfsarch33" || req.AppID != "cursor-global-kb" {
			t.Fatalf("namespace = %s/%s", req.UserID, req.AppID)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "m1"})
	}))
	defer upstream.Close()

	client := NewClient(Options{BaseURL: upstream.URL, APIKey: "test-key", Timeout: time.Second})
	out, err := client.Add(t.Context(), MemoryRequest{
		Messages: []Message{{Role: "user", Content: "remember this"}},
		UserID:   "nfsarch33",
		AppID:    "cursor-global-kb",
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if out["id"] != "m1" {
		t.Fatalf("id = %v", out["id"])
	}
}
