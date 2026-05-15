package cliconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, mode os.FileMode, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return path
}

func TestLoad_MissingFile_ReturnsEnvOnly(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "http://env.local:9999")
	t.Setenv("MEM0_API_KEY", "env-key")
	t.Setenv("MEM0_USER_ID", "")
	t.Setenv("MEM0_APP_ID", "")
	t.Setenv("MEM0_TIMEOUT", "")
	t.Setenv("LOG_LEVEL", "")

	r, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if r.BaseURL != "http://env.local:9999" {
		t.Fatalf("base_url: got %q", r.BaseURL)
	}
	if r.APIKey != "env-key" {
		t.Fatalf("api_key: got %q", r.APIKey)
	}
	if r.SourcePath != "" {
		t.Fatalf("expected empty SourcePath, got %q", r.SourcePath)
	}
}

func TestLoad_EmptyPath_NoEnv_NoFile(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "")
	t.Setenv("MEM0_API_KEY", "")
	r, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if r.BaseURL != "" || r.APIKey != "" {
		t.Fatalf("expected empty, got %+v", r)
	}
}

func TestLoad_InsecurePerm_Rejected(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "")
	t.Setenv("MEM0_API_KEY", "")
	path := writeConfig(t, 0o644, "endpoints:\n  default:\n    base_url: http://x\n")
	_, err := Load(path)
	if err == nil {
		t.Fatalf("expected insecure-perm error")
	}
}

func TestLoad_FileOnlySetsAll(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "")
	t.Setenv("MEM0_API_KEY", "")
	t.Setenv("MEM0_USER_ID", "")
	t.Setenv("MEM0_APP_ID", "")
	t.Setenv("MEM0_TIMEOUT", "")
	t.Setenv("LOG_LEVEL", "")

	body := `endpoints:
  default:
    base_url: "http://127.0.0.1:18888"
    api_key: "file-key"
defaults:
  user_id: "file-user"
  app_id: "file-app"
timeouts:
  request: "12s"
log_level: "debug"
`
	path := writeConfig(t, 0o600, body)
	r, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.BaseURL != "http://127.0.0.1:18888" {
		t.Fatalf("base_url: got %q", r.BaseURL)
	}
	if r.APIKey != "file-key" {
		t.Fatalf("api_key: got %q", r.APIKey)
	}
	if r.UserID != "file-user" || r.AppID != "file-app" {
		t.Fatalf("ids: got %+v", r)
	}
	if r.Timeout != 12*time.Second {
		t.Fatalf("timeout: got %v", r.Timeout)
	}
	if r.LogLevel != "debug" {
		t.Fatalf("log_level: got %q", r.LogLevel)
	}
	if r.SourcePath != path {
		t.Fatalf("source path: got %q", r.SourcePath)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "http://env-wins:1")
	t.Setenv("MEM0_API_KEY", "env-key")
	t.Setenv("MEM0_USER_ID", "env-user")
	t.Setenv("MEM0_APP_ID", "env-app")
	t.Setenv("MEM0_TIMEOUT", "5s")
	t.Setenv("LOG_LEVEL", "warn")

	body := `endpoints:
  default:
    base_url: "http://file:1"
    api_key: "file-key"
defaults:
  user_id: "file-user"
  app_id: "file-app"
timeouts:
  request: "30s"
log_level: "info"
`
	path := writeConfig(t, 0o600, body)
	r, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.BaseURL != "http://env-wins:1" {
		t.Fatalf("env should beat file: got %q", r.BaseURL)
	}
	if r.APIKey != "env-key" {
		t.Fatalf("api_key: %q", r.APIKey)
	}
	if r.UserID != "env-user" || r.AppID != "env-app" {
		t.Fatalf("ids: %+v", r)
	}
	if r.Timeout != 5*time.Second {
		t.Fatalf("timeout: %v", r.Timeout)
	}
	if r.LogLevel != "warn" {
		t.Fatalf("log_level: %q", r.LogLevel)
	}
}

func TestLoad_BadYaml_Errors(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "")
	t.Setenv("MEM0_API_KEY", "")
	path := writeConfig(t, 0o600, "endpoints: : :")
	if _, err := Load(path); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestResolved_Validate(t *testing.T) {
	r := &Resolved{}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty base_url")
	}
	r.BaseURL = "http://x"
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	got, err := expandPath("~/foo/bar.yaml")
	if err != nil {
		t.Fatalf("expandPath: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "foo", "bar.yaml")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}