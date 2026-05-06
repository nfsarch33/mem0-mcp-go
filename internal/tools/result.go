package tools

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func jsonResult(payload any) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(raw)), nil
}

func errResult(err error) *mcp.CallToolResult {
	if err == nil {
		err = errors.New("unknown error")
	}
	return mcp.NewToolResultError(err.Error())
}

func stringArg(args map[string]any, key string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func intArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func mapArg(args map[string]any, key string) map[string]any {
	if value, ok := args[key].(map[string]any); ok {
		return value
	}
	return nil
}
