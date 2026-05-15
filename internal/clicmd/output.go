package clicmd

import (
	"encoding/json"
	"fmt"
	"io"
)

// emit writes payload to w. When jsonOut is true the payload is rendered as
// pretty-printed JSON; otherwise a stable human summary line is written so
// shell pipelines stay grep-friendly.
func emit(w io.Writer, jsonOut bool, payload map[string]any) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	if id, ok := payload["id"].(string); ok && id != "" {
		fmt.Fprintf(w, "ok id=%s\n", id)
		return nil
	}
	if status, ok := payload["status"].(string); ok && status != "" {
		fmt.Fprintf(w, "%s\n", status)
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}