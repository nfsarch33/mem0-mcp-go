package clicmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
)

func cmdSearch(ctx context.Context, deps Deps, args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(deps.Stderr)
	query := fs.String("query", "", "search query (required)")
	user := fs.String("user", "", "user id")
	app := fs.String("app", "", "app id")
	topK := fs.Int("top-k", 0, "max results (0 = server default)")
	jsonOut := fs.Bool("json", false, "emit JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *query == "" {
		return fmt.Errorf("--query is required")
	}
	out, err := deps.Client.Search(ctx, mem0.SearchRequest{
		Query:  *query,
		UserID: chooseString(*user, deps.Config.UserID),
		AppID:  chooseString(*app, deps.Config.AppID),
		Limit:  *topK,
	})
	if err != nil {
		return err
	}
	return emit(deps.Stdout, *jsonOut, out)
}