package mem0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchWirePayload_UserIDAndAppIDInFilters(t *testing.T) {
	t.Parallel()
	payload := SearchRequest{
		Query:  "hello",
		UserID: "u1",
		AppID:  "a1",
		Limit:  5,
	}.wirePayload()

	if payload["query"] != "hello" {
		t.Fatalf("query = %v", payload["query"])
	}
	if payload["limit"] != 5 {
		t.Fatalf("limit = %v", payload["limit"])
	}
	if _, ok := payload["user_id"]; ok {
		t.Fatal("user_id must not be top-level; should be in filters")
	}
	if _, ok := payload["app_id"]; ok {
		t.Fatal("app_id must not be top-level; should be in filters")
	}
	filters, ok := payload["filters"].(map[string]any)
	if !ok {
		t.Fatal("filters is missing or wrong type")
	}
	if filters["user_id"] != "u1" {
		t.Fatalf("filters.user_id = %v", filters["user_id"])
	}
	if filters["app_id"] != "a1" {
		t.Fatalf("filters.app_id = %v", filters["app_id"])
	}
}

func TestSearchWirePayload_MergesExplicitFilters(t *testing.T) {
	t.Parallel()
	payload := SearchRequest{
		Query:   "test",
		UserID:  "u1",
		Filters: map[string]any{"custom_tag": "important"},
	}.wirePayload()

	filters := payload["filters"].(map[string]any)
	if filters["user_id"] != "u1" {
		t.Fatalf("filters.user_id = %v", filters["user_id"])
	}
	if filters["custom_tag"] != "important" {
		t.Fatalf("filters.custom_tag = %v", filters["custom_tag"])
	}
}

func TestSearchWirePayload_OmitsEmptyFields(t *testing.T) {
	t.Parallel()
	payload := SearchRequest{Query: "just a query"}.wirePayload()

	if _, ok := payload["limit"]; ok {
		t.Fatal("zero limit should be omitted")
	}
	if _, ok := payload["filters"]; ok {
		t.Fatal("empty filters should be omitted")
	}
}

func TestSearchSendsFiltersOnWire(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := body["user_id"]; ok {
			t.Fatal("user_id must not appear as top-level in /search body")
		}
		if _, ok := body["app_id"]; ok {
			t.Fatal("app_id must not appear as top-level in /search body")
		}
		filters, ok := body["filters"].(map[string]any)
		if !ok || filters["user_id"] != "nfsarch33" || filters["app_id"] != "cursor-global-kb" {
			t.Fatalf("expected user_id/app_id in filters, got %v", body["filters"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer upstream.Close()

	client := NewClient(Options{BaseURL: upstream.URL, APIKey: "test-key", Timeout: time.Second})
	_, err := client.Search(t.Context(), SearchRequest{
		Query:  "test search",
		UserID: "nfsarch33",
		AppID:  "cursor-global-kb",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
}

func TestUpdateSendsTextFieldNotMemory(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %q, want PUT", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := body["memory"]; ok {
			t.Fatal("must use 'text' field, not 'memory' — OSS rejects 'memory' with 422")
		}
		if body["text"] != "updated content" {
			t.Fatalf("text = %v, want 'updated content'", body["text"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer upstream.Close()

	client := NewClient(Options{BaseURL: upstream.URL, APIKey: "test-key", Timeout: time.Second})
	_, err := client.Update(t.Context(), "mem-id-1", "updated content", nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

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
