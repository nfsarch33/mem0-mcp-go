package clicmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nfsarch33/mem0-mcp-go/internal/cliconfig"
	"github.com/nfsarch33/mem0-mcp-go/internal/mem0"
	"github.com/nfsarch33/mem0-mcp-go/internal/tools"
)

// fakeClient satisfies tools.Mem0Client for unit tests. All methods succeed by
// default; individual cases override specific fn fields.
type fakeClient struct {
	addFn          func(ctx context.Context, req mem0.MemoryRequest) (map[string]any, error)
	searchFn       func(ctx context.Context, req mem0.SearchRequest) (map[string]any, error)
	getFn          func(ctx context.Context, id string) (map[string]any, error)
	getAllFn        func(ctx context.Context, userID, appID string, limit int) (map[string]any, error)
	updateFn       func(ctx context.Context, id, memory string, metadata map[string]any) (map[string]any, error)
	deleteFn       func(ctx context.Context, id string) (map[string]any, error)
	historyFn      func(ctx context.Context, id string) (map[string]any, error)
	doctorFn       func(ctx context.Context) error
	listEntitiesFn func(ctx context.Context) (map[string]any, error)
}

func (f *fakeClient) Add(ctx context.Context, req mem0.MemoryRequest) (map[string]any, error) {
	if f.addFn != nil {
		return f.addFn(ctx, req)
	}
	return map[string]any{"id": "test-id-1"}, nil
}
func (f *fakeClient) Search(ctx context.Context, req mem0.SearchRequest) (map[string]any, error) {
	if f.searchFn != nil {
		return f.searchFn(ctx, req)
	}
	return map[string]any{"results": []any{}}, nil
}
func (f *fakeClient) Get(ctx context.Context, id string) (map[string]any, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return map[string]any{"id": id, "memory": "test memory"}, nil
}
func (f *fakeClient) GetAll(ctx context.Context, userID, appID string, limit int) (map[string]any, error) {
	if f.getAllFn != nil {
		return f.getAllFn(ctx, userID, appID, limit)
	}
	return map[string]any{"results": []any{}}, nil
}
func (f *fakeClient) Update(ctx context.Context, id, memory string, metadata map[string]any) (map[string]any, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, memory, metadata)
	}
	return map[string]any{"id": id}, nil
}
func (f *fakeClient) Delete(ctx context.Context, id string) (map[string]any, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return map[string]any{"status": "deleted"}, nil
}
func (f *fakeClient) History(ctx context.Context, id string) (map[string]any, error) {
	if f.historyFn != nil {
		return f.historyFn(ctx, id)
	}
	return map[string]any{"history": []any{}}, nil
}
func (f *fakeClient) Doctor(ctx context.Context) error {
	if f.doctorFn != nil {
		return f.doctorFn(ctx)
	}
	return nil
}
func (f *fakeClient) ListEntities(ctx context.Context) (map[string]any, error) {
	if f.listEntitiesFn != nil {
		return f.listEntitiesFn(ctx)
	}
	return map[string]any{"entities": []any{}}, nil
}

var _ tools.Mem0Client = (*fakeClient)(nil)

func newTestDeps(fc *fakeClient) (Deps, *bytes.Buffer, *bytes.Buffer) {
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	return Deps{
		Client: fc,
		Config: &cliconfig.Resolved{
			BaseURL: "http://localhost:8888",
			UserID:  "test-user",
			AppID:   "test-app",
		},
		Stdout: out,
		Stderr: errBuf,
	}, out, errBuf
}

// Run dispatch tests — requires a real config load so we inject MEM0_BASE_URL
// to satisfy Validate() and point --config at a nonexistent path (missing file
// is allowed by cliconfig.Load).

func TestRun_EmptyArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), []string{}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
}

func TestRun_HelpFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), []string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("want exit 0 for --help, got %d", code)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "http://localhost:8888")
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), []string{"not-a-command"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "unknown subcommand") {
		t.Fatalf("want 'unknown subcommand' in stderr, got %q", errBuf.String())
	}
}

// Subcommand function tests (internal package access).

func TestCmdHealth_OK(t *testing.T) {
	deps, out, _ := newTestDeps(&fakeClient{})
	if err := cmdHealth(context.Background(), deps, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("want 'ok' in output, got %q", out.String())
	}
}

func TestCmdHealth_JSONOut(t *testing.T) {
	deps, out, _ := newTestDeps(&fakeClient{})
	if err := cmdHealth(context.Background(), deps, []string{"--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"status"`) {
		t.Fatalf("want JSON with status key, got %q", out.String())
	}
	if !strings.Contains(out.String(), `"base_url"`) {
		t.Fatalf("want JSON with base_url key, got %q", out.String())
	}
}

func TestCmdHealth_DoctorError(t *testing.T) {
	fc := &fakeClient{doctorFn: func(_ context.Context) error { return errors.New("connection refused") }}
	deps, _, _ := newTestDeps(fc)
	err := cmdHealth(context.Background(), deps, []string{})
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("want connection refused error, got %v", err)
	}
}

func TestCmdAdd_MissingText(t *testing.T) {
	deps, _, _ := newTestDeps(&fakeClient{})
	err := cmdAdd(context.Background(), deps, []string{})
	if err == nil {
		t.Fatal("want error for missing --text")
	}
}

func TestCmdAdd_OK(t *testing.T) {
	var gotText string
	fc := &fakeClient{addFn: func(_ context.Context, req mem0.MemoryRequest) (map[string]any, error) {
		if len(req.Messages) > 0 {
			gotText = req.Messages[0].Content
		}
		return map[string]any{"id": "added-id"}, nil
	}}
	deps, out, _ := newTestDeps(fc)
	if err := cmdAdd(context.Background(), deps, []string{"--text", "hello world"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotText != "hello world" {
		t.Fatalf("want text 'hello world', got %q", gotText)
	}
	if !strings.Contains(out.String(), "added-id") {
		t.Fatalf("want id in output, got %q", out.String())
	}
}

func TestCmdAdd_ClientError(t *testing.T) {
	fc := &fakeClient{addFn: func(_ context.Context, _ mem0.MemoryRequest) (map[string]any, error) {
		return nil, errors.New("upstream 500")
	}}
	deps, _, _ := newTestDeps(fc)
	err := cmdAdd(context.Background(), deps, []string{"--text", "fail"})
	if err == nil || !strings.Contains(err.Error(), "upstream 500") {
		t.Fatalf("want upstream error, got %v", err)
	}
}

func TestCmdSearch_MissingQuery(t *testing.T) {
	deps, _, _ := newTestDeps(&fakeClient{})
	err := cmdSearch(context.Background(), deps, []string{})
	if err == nil {
		t.Fatal("want error for missing --query")
	}
}

func TestCmdSearch_OK(t *testing.T) {
	fc := &fakeClient{searchFn: func(_ context.Context, req mem0.SearchRequest) (map[string]any, error) {
		if req.Query != "golang" {
			t.Errorf("want query 'golang', got %q", req.Query)
		}
		return map[string]any{"results": []any{"hit1"}}, nil
	}}
	deps, out, _ := newTestDeps(fc)
	if err := cmdSearch(context.Background(), deps, []string{"--query", "golang"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "hit1") {
		t.Fatalf("want result in output, got %q", out.String())
	}
}

func TestCmdSearch_ForwardsUserApp(t *testing.T) {
	var gotUser, gotApp string
	fc := &fakeClient{searchFn: func(_ context.Context, req mem0.SearchRequest) (map[string]any, error) {
		gotUser = req.UserID
		gotApp = req.AppID
		return map[string]any{"results": []any{}}, nil
	}}
	deps, _, _ := newTestDeps(fc)
	// defaults from config
	if err := cmdSearch(context.Background(), deps, []string{"--query", "x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUser != "test-user" {
		t.Fatalf("want user 'test-user', got %q", gotUser)
	}
	if gotApp != "test-app" {
		t.Fatalf("want app 'test-app', got %q", gotApp)
	}
}

func TestCmdGet_MissingID(t *testing.T) {
	deps, _, _ := newTestDeps(&fakeClient{})
	if err := cmdGet(context.Background(), deps, []string{}); err == nil {
		t.Fatal("want error for missing --memory-id")
	}
}

func TestCmdGet_OK(t *testing.T) {
	deps, out, _ := newTestDeps(&fakeClient{})
	if err := cmdGet(context.Background(), deps, []string{"--memory-id", "mem-123"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "mem-123") {
		t.Fatalf("want id in output, got %q", out.String())
	}
}

func TestCmdGetMemories_OK(t *testing.T) {
	deps, _, _ := newTestDeps(&fakeClient{})
	if err := cmdGetMemories(context.Background(), deps, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdGetMemories_CustomLimit(t *testing.T) {
	var gotLimit int
	fc := &fakeClient{getAllFn: func(_ context.Context, _, _ string, limit int) (map[string]any, error) {
		gotLimit = limit
		return map[string]any{"results": []any{}}, nil
	}}
	deps, _, _ := newTestDeps(fc)
	if err := cmdGetMemories(context.Background(), deps, []string{"--limit", "42"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 42 {
		t.Fatalf("want limit 42, got %d", gotLimit)
	}
}

func TestCmdDelete_MissingID(t *testing.T) {
	deps, _, _ := newTestDeps(&fakeClient{})
	if err := cmdDelete(context.Background(), deps, []string{}); err == nil {
		t.Fatal("want error for missing --memory-id")
	}
}

func TestCmdDelete_OK(t *testing.T) {
	var deletedID string
	fc := &fakeClient{deleteFn: func(_ context.Context, id string) (map[string]any, error) {
		deletedID = id
		return map[string]any{"status": "deleted"}, nil
	}}
	deps, out, _ := newTestDeps(fc)
	if err := cmdDelete(context.Background(), deps, []string{"--memory-id", "del-456"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "del-456" {
		t.Fatalf("want id 'del-456', got %q", deletedID)
	}
	if !strings.Contains(out.String(), "deleted") {
		t.Fatalf("want 'deleted' in output, got %q", out.String())
	}
}

func TestCmdListEntities_OK(t *testing.T) {
	fc := &fakeClient{listEntitiesFn: func(_ context.Context) (map[string]any, error) {
		return map[string]any{"entities": []any{"user1", "user2"}}, nil
	}}
	deps, out, _ := newTestDeps(fc)
	if err := cmdListEntities(context.Background(), deps, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "user1") {
		t.Fatalf("want entity in output, got %q", out.String())
	}
}

func TestCmdListEntities_Error(t *testing.T) {
	fc := &fakeClient{listEntitiesFn: func(_ context.Context) (map[string]any, error) {
		return nil, errors.New("server error")
	}}
	deps, _, _ := newTestDeps(fc)
	err := cmdListEntities(context.Background(), deps, []string{})
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("want server error, got %v", err)
	}
}
