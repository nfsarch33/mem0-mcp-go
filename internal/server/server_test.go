package server

import (
	"context"
	"encoding/json"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

type fakeClient struct{}

func (fakeClient) Add(context.Context, mem0.MemoryRequest) (map[string]any, error) {
	return map[string]any{"status": "ok"}, nil
}
func (fakeClient) Search(context.Context, mem0.SearchRequest) (map[string]any, error) {
	return map[string]any{"results": []any{
		map[string]any{"id": "m1", "memory": "test memory", "score": 0.95},
	}}, nil
}
func (fakeClient) Get(context.Context, string) (map[string]any, error) {
	return map[string]any{"id": "m1", "memory": "test memory"}, nil
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

func TestServerRegistersTenTools(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry(fakeClient{}, tools.Defaults{UserID: "u", AppID: "a"})
	srv := New(reg, nil, "test")
	if srv.ToolCount() != 10 {
		t.Fatalf("ToolCount() = %d, want 10", srv.ToolCount())
	}
	if srv.MCPServer() == nil {
		t.Fatal("MCPServer() is nil")
	}
}

func TestServer_CursorSmokeListsToolsAndSearchOK(t *testing.T) {
	t.Parallel()
	reg := tools.NewRegistry(fakeClient{}, tools.Defaults{UserID: "u", AppID: "a"})
	srv := New(reg, nil, "test")

	client, err := mcpclient.NewInProcessClient(srv.MCPServer())
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: "cursor-smoke", Version: "1.0"}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	if _, err := client.Initialize(ctx, initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// tools/list round-trip
	listReq := mcp.ListToolsRequest{}
	listResult, err := client.ListTools(ctx, listReq)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	cursorTools := map[string]bool{
		"memory_search": false,
		"memory_write":  false,
		"memory_read":   false,
	}
	for _, tool := range listResult.Tools {
		if _, ok := cursorTools[tool.Name]; ok {
			cursorTools[tool.Name] = true
		}
	}
	for name, found := range cursorTools {
		if !found {
			t.Errorf("tools/list missing Cursor tool %q", name)
		}
	}

	// tools/call round-trip: memory_search
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = "memory_search"
	callReq.Params.Arguments = map[string]any{"query": "test query"}
	callResult, err := client.CallTool(ctx, callReq)
	if err != nil {
		t.Fatalf("CallTool(memory_search): %v", err)
	}
	if callResult.IsError {
		t.Fatalf("memory_search returned error: %v", callResult.Content)
	}
	if len(callResult.Content) == 0 {
		t.Fatal("memory_search returned empty content")
	}
	text, ok := callResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want mcp.TextContent", callResult.Content[0])
	}
	var searchOut map[string]any
	if err := json.Unmarshal([]byte(text.Text), &searchOut); err != nil {
		t.Fatalf("unmarshal search output: %v", err)
	}
	results, ok := searchOut["results"]
	if !ok {
		t.Fatal("search output missing 'results' key")
	}
	arr, ok := results.([]any)
	if !ok || len(arr) == 0 {
		t.Fatal("search output 'results' should be a non-empty array")
	}
}
