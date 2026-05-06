package tools

import (
	"context"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
)

type Mem0Client interface {
	Add(ctx context.Context, req mem0.MemoryRequest) (map[string]any, error)
	Search(ctx context.Context, req mem0.SearchRequest) (map[string]any, error)
	GetAll(ctx context.Context, userID, appID string, limit int) (map[string]any, error)
	Update(ctx context.Context, id, memory string, metadata map[string]any) (map[string]any, error)
	Delete(ctx context.Context, id string) (map[string]any, error)
	History(ctx context.Context, id string) (map[string]any, error)
	Doctor(ctx context.Context) error
}

type Defaults struct {
	UserID string
	AppID  string
}

type Registry struct {
	client   Mem0Client
	defaults Defaults
}

func NewRegistry(client Mem0Client, defaults Defaults) *Registry {
	return &Registry{client: client, defaults: defaults}
}

func (r *Registry) Tools() []RegisteredTool {
	return []RegisteredTool{
		{Tool: r.addTool(), Handler: r.handleAdd},
		{Tool: r.searchTool(), Handler: r.handleSearch},
		{Tool: r.getAllTool(), Handler: r.handleGetAll},
		{Tool: r.updateTool(), Handler: r.handleUpdate},
		{Tool: r.deleteTool(), Handler: r.handleDelete},
		{Tool: r.historyTool(), Handler: r.handleHistory},
		{Tool: r.doctorTool(), Handler: r.handleDoctor},
	}
}

type RegisteredTool struct {
	Tool    mcp.Tool
	Handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

func (r *Registry) addTool() mcp.Tool {
	return mcp.NewTool("mem0_add",
		mcp.WithDescription("Add a memory to self-hosted Mem0 OSS."),
		mcp.WithString("text", mcp.Required(), mcp.Description("Memory text to store.")),
		mcp.WithString("user_id", mcp.Description("Mem0 user id; defaults to MEM0_USER_ID.")),
		mcp.WithString("app_id", mcp.Description("Mem0 app id; defaults to MEM0_APP_ID.")),
		mcp.WithObject("metadata", mcp.Description("Optional metadata object.")),
		mcp.WithBoolean("infer", mcp.Description("Whether Mem0 should infer structured memories. Defaults to true.")),
	)
}

func (r *Registry) searchTool() mcp.Tool {
	return mcp.NewTool("mem0_search",
		mcp.WithDescription("Search self-hosted Mem0 OSS memories."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query.")),
		mcp.WithString("user_id", mcp.Description("Mem0 user id; defaults to MEM0_USER_ID.")),
		mcp.WithString("app_id", mcp.Description("Mem0 app id; defaults to MEM0_APP_ID.")),
		mcp.WithNumber("limit", mcp.Description("Maximum results.")),
		mcp.WithObject("filters", mcp.Description("Optional Mem0 filter object.")),
	)
}

func (r *Registry) getAllTool() mcp.Tool {
	return mcp.NewTool("mem0_get_all",
		mcp.WithDescription("List memories from self-hosted Mem0 OSS."),
		mcp.WithString("user_id", mcp.Description("Mem0 user id; defaults to MEM0_USER_ID.")),
		mcp.WithString("app_id", mcp.Description("Mem0 app id; defaults to MEM0_APP_ID.")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows.")),
	)
}

func (r *Registry) updateTool() mcp.Tool {
	return mcp.NewTool("mem0_update",
		mcp.WithDescription("Update a memory in self-hosted Mem0 OSS."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory id.")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Replacement memory text.")),
		mcp.WithObject("metadata", mcp.Description("Optional metadata object.")),
	)
}

func (r *Registry) deleteTool() mcp.Tool {
	return mcp.NewTool("mem0_delete",
		mcp.WithDescription("Delete a memory from self-hosted Mem0 OSS."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory id.")),
	)
}

func (r *Registry) historyTool() mcp.Tool {
	return mcp.NewTool("mem0_history",
		mcp.WithDescription("Read memory history from self-hosted Mem0 OSS."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory id.")),
	)
}

func (r *Registry) doctorTool() mcp.Tool {
	return mcp.NewTool("mem0_doctor",
		mcp.WithDescription("Check connectivity to the configured self-hosted Mem0 OSS endpoint."),
	)
}

func (r *Registry) handleAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	text := stringArg(args, "text")
	if text == "" {
		return errResult(errors.New("text is required")), nil
	}
	infer := true
	if value, ok := args["infer"].(bool); ok {
		infer = value
	}
	out, err := r.client.Add(ctx, mem0.MemoryRequest{
		Messages: []mem0.Message{{Role: "user", Content: text}},
		UserID:   r.userID(args),
		AppID:    r.appID(args),
		Metadata: mapArg(args, "metadata"),
		Infer:    &infer,
	})
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(out)
}

func (r *Registry) handleSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	query := stringArg(args, "query")
	if query == "" {
		return errResult(errors.New("query is required")), nil
	}
	out, err := r.client.Search(ctx, mem0.SearchRequest{
		Query:   query,
		UserID:  r.userID(args),
		AppID:   r.appID(args),
		Limit:   intArg(args, "limit"),
		Filters: mapArg(args, "filters"),
	})
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(out)
}

func (r *Registry) handleGetAll(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	out, err := r.client.GetAll(ctx, r.userID(args), r.appID(args), intArg(args, "limit"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(out)
}

func (r *Registry) handleUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	out, err := r.client.Update(ctx, stringArg(args, "id"), stringArg(args, "text"), mapArg(args, "metadata"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(out)
}

func (r *Registry) handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out, err := r.client.Delete(ctx, stringArg(req.GetArguments(), "id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(out)
}

func (r *Registry) handleHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out, err := r.client.History(ctx, stringArg(req.GetArguments(), "id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(out)
}

func (r *Registry) handleDoctor(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := r.client.Doctor(ctx); err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "ok"})
}

func (r *Registry) userID(args map[string]any) string {
	if value := stringArg(args, "user_id"); value != "" {
		return value
	}
	return r.defaults.UserID
}

func (r *Registry) appID(args map[string]any) string {
	if value := stringArg(args, "app_id"); value != "" {
		return value
	}
	return r.defaults.AppID
}
