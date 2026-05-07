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

	client := startCursorSmokeClient(t, srv)
	defer client.Close()

	assertCursorToolsListed(t, client)
	assertMemorySearchCallOK(t, client)
}

func startCursorSmokeClient(t *testing.T, srv *Server) *mcpclient.Client {
	t.Helper()
	client, err := mcpclient.NewInProcessClient(srv.MCPServer())
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}

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
	return client
}

func assertCursorToolsListed(t *testing.T, client *mcpclient.Client) {
	t.Helper()
	listReq := mcp.ListToolsRequest{}
	listResult, err := client.ListTools(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := toolNameSet(listResult.Tools)
	required := []string{
		"memory_search",
		"memory_write",
		"memory_read",
	}
	for _, name := range required {
		if !found[name] {
			t.Errorf("tools/list missing Cursor tool %q", name)
		}
	}
}

func toolNameSet(listed []mcp.Tool) map[string]bool {
	found := map[string]bool{
		"memory_search": false,
		"memory_write":  false,
		"memory_read":   false,
	}
	for _, tool := range listed {
		if _, ok := found[tool.Name]; ok {
			found[tool.Name] = true
		}
	}
	return found
}

func assertMemorySearchCallOK(t *testing.T, client *mcpclient.Client) {
	t.Helper()
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = "memory_search"
	callReq.Params.Arguments = map[string]any{"query": "test query"}
	callResult, err := client.CallTool(context.Background(), callReq)
	if err != nil {
		t.Fatalf("CallTool(memory_search): %v", err)
	}
	if callResult.IsError {
		t.Fatalf("memory_search returned error: %v", callResult.Content)
	}
	if len(callResult.Content) == 0 {
		t.Fatal("memory_search returned empty content")
	}
	searchOut := decodeFirstTextContent(t, callResult)
	assertNonEmptyResults(t, searchOut)
}

func decodeFirstTextContent(t *testing.T, callResult *mcp.CallToolResult) map[string]any {
	t.Helper()
	text, ok := callResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want mcp.TextContent", callResult.Content[0])
	}
	var searchOut map[string]any
	if err := json.Unmarshal([]byte(text.Text), &searchOut); err != nil {
		t.Fatalf("unmarshal search output: %v", err)
	}
	return searchOut
}

func assertNonEmptyResults(t *testing.T, searchOut map[string]any) {
	t.Helper()
	results, ok := searchOut["results"]
	if !ok {
		t.Fatal("search output missing 'results' key")
	}
	arr, ok := results.([]any)
	if !ok || len(arr) == 0 {
		t.Fatal("search output 'results' should be a non-empty array")
	}
}
