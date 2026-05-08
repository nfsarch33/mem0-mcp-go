package config

import (
	"testing"
)

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
