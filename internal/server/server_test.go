package server

import (
	"context"
	"testing"

	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

type fakeClient struct{}

func (fakeClient) Add(context.Context, mem0.MemoryRequest) (map[string]any, error) {
	return map[string]any{"status": "ok"}, nil
}
func (fakeClient) Search(context.Context, mem0.SearchRequest) (map[string]any, error) {
	return map[string]any{"results": []any{}}, nil
}
func (fakeClient) GetAll(context.Context, string, string, int) (map[string]any, error) {
	return map[string]any{"memories": []any{}}, nil
}
func (fakeClient) Update(context.Context, string, string, map[string]any) (map[string]any, error) {
	return map[string]any{"status": "updated"}, nil
}
func (fakeClient) Delete(context.Context, string) (map[string]any, error) {
	return map[string]any{"status": "deleted"}, nil
}
func (fakeClient) History(context.Context, string) (map[string]any, error) {
	return map[string]any{"history": []any{}}, nil
}
func (fakeClient) Doctor(context.Context) error { return nil }

func TestServerRegistersSevenTools(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry(fakeClient{}, tools.Defaults{UserID: "u", AppID: "a"})
	srv := New(reg, nil, "test")
	if srv.ToolCount() != 7 {
		t.Fatalf("ToolCount() = %d, want 7", srv.ToolCount())
	}
	if srv.MCPServer() == nil {
		t.Fatal("MCPServer() is nil")
	}
}
