package middleware_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nfsarch33/mem0-mcp-go/internal/middleware"
)

var mem0StripFields = []string{
	"metadata", "hash", "created_at", "updated_at",
	"user_id", "app_id",
}

var requiredFields = []string{"memory", "score"}

func mem0Filter() *middleware.FieldFilter {
	return middleware.NewFieldFilter(
		middleware.WithStripFields(mem0StripFields...),
		middleware.WithMaxArrayLen(20),
	)
}

func generateMem0Payloads(n int) [][]byte {
	payloads := make([][]byte, n)
	for i := 0; i < n; i++ {
		results := make([]any, 1+(i%5))
		for j := range results {
			results[j] = map[string]any{
				"memory":     fmt.Sprintf("memory item %d-%d: important fact about topic %d", i, j, j),
				"score":      0.85 + float64(j)*0.02,
				"metadata":   map[string]any{"source": "auto", "model": "gpt-4", "extra_field": strings.Repeat("x", 50+i)},
				"hash":       fmt.Sprintf("hash-%d-%d", i, j),
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-05-14T00:00:00Z",
				"user_id":    "nfsarch33",
				"app_id":     "cursor-global-kb",
			}
		}
		payload := map[string]any{"results": results}
		data, _ := json.Marshal(payload)
		payloads[i] = data
	}
	return payloads
}

func TestAccuracy_NoRequiredFieldStripped(t *testing.T) {
	t.Parallel()
	f := mem0Filter()
	payloads := generateMem0Payloads(50)

	falseStrips := 0
	total := 0

	for i, payload := range payloads {
		result := f.Filter(payload)
		var out map[string]any
		if err := json.Unmarshal(result, &out); err != nil {
			t.Fatalf("payload %d: invalid JSON output: %v", i, err)
		}

		results, ok := out["results"].([]any)
		if !ok {
			t.Fatalf("payload %d: results field missing", i)
		}

		for j, item := range results {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			total++
			for _, field := range requiredFields {
				if _, exists := obj[field]; !exists {
					falseStrips++
					t.Errorf("payload %d item %d: required field %q was stripped", i, j, field)
				}
			}
		}
	}

	if total == 0 {
		t.Fatal("no items checked")
	}
	rate := float64(falseStrips) / float64(total*len(requiredFields)) * 100
	if rate > 1.0 {
		t.Errorf("false-strip rate %.2f%% > 1%% threshold", rate)
	}
	t.Logf("accuracy: %d items checked, %d false strips, rate=%.2f%%", total, falseStrips, rate)
}

func TestAccuracy_SavingsAbove30Percent(t *testing.T) {
	t.Parallel()
	f := mem0Filter()
	payloads := generateMem0Payloads(50)

	belowThreshold := 0
	for i, payload := range payloads {
		result := f.Filter(payload)
		savings := middleware.TokenSavings(payload, result)
		if savings < 30 {
			belowThreshold++
			t.Logf("payload %d: savings %.1f%% < 30%%", i, savings)
		}
	}

	if belowThreshold > 0 {
		t.Errorf("%d/%d payloads had <30%% savings", belowThreshold, len(payloads))
	}
}

func TestAccuracy_NestedStripCorrect(t *testing.T) {
	t.Parallel()
	f := middleware.NewFieldFilter(
		middleware.WithStripFields("result.metadata", "result.internal_state"),
	)

	input := map[string]any{
		"result": map[string]any{
			"content":        "preserved",
			"metadata":       map[string]any{"source": "strip-me", "nested": "deep"},
			"internal_state": "strip-this-too",
		},
		"metadata": "top-level-should-stay",
	}

	data, _ := json.Marshal(input)
	result := f.Filter(data)

	var out map[string]any
	json.Unmarshal(result, &out)

	if out["metadata"] != "top-level-should-stay" {
		t.Error("top-level metadata with same name but different path should be preserved")
	}

	nested := out["result"].(map[string]any)
	if nested["content"] != "preserved" {
		t.Error("content should be preserved")
	}
	if _, ok := nested["metadata"]; ok {
		t.Error("result.metadata should be stripped")
	}
	if _, ok := nested["internal_state"]; ok {
		t.Error("result.internal_state should be stripped")
	}
}

func TestAccuracy_EdgeCase_EmptyResults(t *testing.T) {
	t.Parallel()
	f := mem0Filter()
	input := `{"results":[]}`
	result := f.Filter([]byte(input))
	if string(result) != `{"results":[]}` {
		t.Errorf("empty results should stay empty, got %s", result)
	}
}

func TestAccuracy_EdgeCase_NullFields(t *testing.T) {
	t.Parallel()
	f := mem0Filter()
	input := `{"results":[{"memory":"test","score":null,"metadata":null}]}`
	result := f.Filter([]byte(input))

	var out map[string]any
	json.Unmarshal(result, &out)
	results := out["results"].([]any)
	first := results[0].(map[string]any)
	if _, ok := first["memory"]; !ok {
		t.Error("memory should be preserved even with null sibling fields")
	}
}

func TestAccuracy_EdgeCase_UnicodeContent(t *testing.T) {
	t.Parallel()
	f := mem0Filter()
	input := `{"results":[{"memory":"日本語テスト 中文测试 한국어","score":0.99,"metadata":{"lang":"multi"},"hash":"utf8"}]}`
	result := f.Filter([]byte(input))

	var out map[string]any
	json.Unmarshal(result, &out)
	results := out["results"].([]any)
	first := results[0].(map[string]any)
	mem := first["memory"].(string)
	if !strings.Contains(mem, "日本語") {
		t.Error("unicode content should be preserved")
	}
}
