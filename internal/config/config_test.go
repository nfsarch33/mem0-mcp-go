package config

import (
	"strings"
	"testing"
)

// TestLoad_DefaultsAreGeneric asserts that Load() with no MEM0_* env
// vars set returns generic defaults that do not encode any
// operator-specific topology (loopback IP, personal tunnel port,
// GitHub login, KB repo name). Operators must explicitly set
// MEM0_BASE_URL in their deploy environment.
//
// This is the v321-5 acceptance gate: it backstops the
// runx public-repo-gate check by encoding the leak categories as a
// unit-test assertion that runs in CI.
func TestLoad_DefaultsAreGeneric(t *testing.T) {
	envKeys := []string{
		"MEM0_BASE_URL",
		"MEM0_API_KEY",
		"MEM0_USER_ID",
		"MEM0_DEFAULT_USER_ID",
		"MEM0_APP_ID",
		"MEM0_DEFAULT_APP_ID",
		"MCP_TRANSPORT",
		"MCP_SSE_ADDR",
		"MEM0_TIMEOUT",
		"LOG_LEVEL",
		"MEM0_DUAL_WRITE",
		"MEM0_CLOUD_URL",
		"MEM0_CLOUD_API_KEY",
		"MEM0_READ_SOURCE",
		"MEM0_BACKUP_URL",
		"MEM0_BACKUP_API_KEY",
	}
	for _, k := range envKeys {
		t.Setenv(k, "")
	}

	cfg := Load()

	forbidden := []string{
		"127.0.0.1",
		"18888",
		"nfsarch33",
		"cursor-global-kb",
	}
	fields := map[string]string{
		"BaseURL": cfg.BaseURL,
		"UserID":  cfg.UserID,
		"AppID":   cfg.AppID,
	}
	for fieldName, value := range fields {
		for _, term := range forbidden {
			if strings.Contains(value, term) {
				t.Errorf("Config.%s default %q contains forbidden term %q (leaks operator topology)",
					fieldName, value, term)
			}
		}
	}

	if cfg.BaseURL != "" {
		t.Errorf("BaseURL default = %q, want empty (operators must set MEM0_BASE_URL explicitly)", cfg.BaseURL)
	}
	if cfg.UserID != "default-user" {
		t.Errorf("UserID default = %q, want default-user", cfg.UserID)
	}
	if cfg.AppID != "default-app" {
		t.Errorf("AppID default = %q, want default-app", cfg.AppID)
	}
}

func TestLoad_DualWriteDefaults(t *testing.T) {
	t.Parallel()
	cfg := Load()

	if cfg.DualWrite {
		t.Fatal("DualWrite should default to false")
	}
	if cfg.CloudURL != "https://api.mem0.ai" {
		t.Fatalf("CloudURL = %q, want https://api.mem0.ai", cfg.CloudURL)
	}
	if cfg.ReadSource != "oss" {
		t.Fatalf("ReadSource = %q, want oss", cfg.ReadSource)
	}
}

func TestLoad_CompatibilityFallbacks(t *testing.T) {
	t.Setenv("MEM0_USER_ID", "")
	t.Setenv("MEM0_DEFAULT_USER_ID", "compat-user")
	t.Setenv("MEM0_APP_ID", "")
	t.Setenv("MEM0_DEFAULT_APP_ID", "compat-app")

	cfg := Load()

	if cfg.UserID != "compat-user" {
		t.Fatalf("UserID = %q, want compat-user", cfg.UserID)
	}
	if cfg.AppID != "compat-app" {
		t.Fatalf("AppID = %q, want compat-app", cfg.AppID)
	}
}

func TestDualWriteEnabled_RequiresURLAndKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     Config
		enabled bool
	}{
		{"all set", Config{DualWrite: true, CloudURL: "https://api.mem0.ai", CloudAPIKey: "key"}, true},
		{"missing key", Config{DualWrite: true, CloudURL: "https://api.mem0.ai"}, false},
		{"missing url", Config{DualWrite: true, CloudAPIKey: "key"}, false},
		{"disabled", Config{DualWrite: false, CloudURL: "https://api.mem0.ai", CloudAPIKey: "key"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.cfg.DualWriteEnabled(); got != tt.enabled {
				t.Fatalf("DualWriteEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

func TestBackupEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     Config
		enabled bool
	}{
		{"both set", Config{BackupURL: "http://backup", BackupAPIKey: "key"}, true},
		{"missing key", Config{BackupURL: "http://backup"}, false},
		{"missing url", Config{BackupAPIKey: "key"}, false},
		{"both empty", Config{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.cfg.BackupEnabled(); got != tt.enabled {
				t.Fatalf("BackupEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

func TestGetenvBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback bool
		want     bool
	}{
		{"empty_false", "", false, false},
		{"empty_true", "", true, true},
		{"true", "true", false, true},
		{"True", "True", false, true},
		{"TRUE", "TRUE", false, true},
		{"one", "1", false, true},
		{"yes", "yes", false, true},
		{"on", "on", false, true},
		{"false", "false", true, false},
		{"no", "no", true, false},
		{"zero", "0", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_" + tt.name
			if tt.input != "" {
				t.Setenv(key, tt.input)
			}
			if got := getenvBool(key, tt.fallback); got != tt.want {
				t.Fatalf("getenvBool(%q, %v) = %v, want %v", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}
