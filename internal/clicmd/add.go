package clicmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
)

func cmdAdd(ctx context.Context, deps Deps, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	text := fs.String("text", "", "memory text to store (required)")
	user := fs.String("user", "", "user id; defaults to config.defaults.user_id")
	app := fs.String("app", "", "app id; defaults to config.defaults.app_id")
	noInfer := fs.Bool("no-infer", false, "store verbatim without LLM inference (faster, avoids 60s retry)")
	jsonOut := fs.Bool("json", false, "emit JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *text == "" {
		return fmt.Errorf("--text is required")
	}

	infer := !*noInfer
	out, err := deps.Client.Add(ctx, mem0.MemoryRequest{
		Messages: []mem0.Message{{Role: "user", Content: *text}},
		UserID:   chooseString(*user, deps.Config.UserID),
		AppID:    chooseString(*app, deps.Config.AppID),
		Infer:    &infer,
	})
	if err != nil {
		return err
	}
	return emit(deps.Stdout, *jsonOut, out)
}

func chooseString(flagVal, fallback string) string {
	if flagVal != "" {
		return flagVal
	}
	return fallback
}