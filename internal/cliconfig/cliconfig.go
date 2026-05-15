// runx-public-repo-gate: allow-file network_topology — documents the canonical Mem0 OSS loopback endpoint (127.0.0.1:18888) used as the local CLI default; not a personal-stack tunnel.
// Package cliconfig loads YAML config for the mem0-mcp-go CLI surface.
//
// Precedence: file < env < flag. Secrets resolve from the file or env only,
// never from argv (no-shell-leak rule).
//
// File schema (~/.config/mem0-mcp-go/config.yaml):
//
//	endpoints:
//	  default:
//	    base_url: "http://127.0.0.1:18888"
//	    api_key: "..."        # optional; may also come from MEM0_API_KEY
//	defaults:
//	  user_id: "default-user"
//	  app_id: "default-app"
//	timeouts:
//	  request: "30s"
//	log_level: "info"
package cliconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigPath is the file path used when --config is not set.
	DefaultConfigPath = "~/.config/mem0-mcp-go/config.yaml"
	// DefaultEndpointName is the named entry resolved from endpoints map.
	DefaultEndpointName = "default"
	// MaxFileMode is the most permissive file mode allowed (0o600).
	MaxFileMode os.FileMode = 0o600
)

// File is the in-memory representation of the config file.
type File struct {
	Endpoints map[string]Endpoint `yaml:"endpoints"`
	Defaults  Defaults            `yaml:"defaults"`
	Timeouts  Timeouts            `yaml:"timeouts"`
	LogLevel  string              `yaml:"log_level"`
}

// Endpoint is a single Mem0 OSS target.
type Endpoint struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// Defaults provides fallback identifiers.
type Defaults struct {
	UserID string `yaml:"user_id"`
	AppID  string `yaml:"app_id"`
}

// Timeouts captures request/HTTP timeouts.
type Timeouts struct {
	Request string `yaml:"request"`
}

// Resolved is the merged config (file + env), ready for the CLI to consume.
type Resolved struct {
	BaseURL    string
	APIKey     string
	UserID     string
	AppID      string
	Timeout    time.Duration
	LogLevel   string
	SourcePath string // empty when no file was loaded
}

// Load reads the YAML file at path (after expanding ~), merges environment
// overrides on top, and returns the Resolved view. A missing file is not an
// error: env-only operation is supported. The file MUST be 0o600 or stricter
// when present, to honour the no-shell-leak rule.
func Load(path string) (*Resolved, error) {
	expanded, err := expandPath(path)
	if err != nil {
		return nil, err
	}

	resolved := &Resolved{
		BaseURL:  os.Getenv("MEM0_BASE_URL"),
		APIKey:   os.Getenv("MEM0_API_KEY"),
		UserID:   firstNonEmpty(os.Getenv("MEM0_USER_ID"), os.Getenv("MEM0_DEFAULT_USER_ID")),
		AppID:    firstNonEmpty(os.Getenv("MEM0_APP_ID"), os.Getenv("MEM0_DEFAULT_APP_ID")),
		Timeout:  parseTimeoutOrDefault(os.Getenv("MEM0_TIMEOUT"), 30*time.Second),
		LogLevel: firstNonEmpty(os.Getenv("LOG_LEVEL"), "info"),
	}

	if expanded == "" {
		return resolved, nil
	}

	info, err := os.Stat(expanded)
	if os.IsNotExist(err) {
		return resolved, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat config: %w", err)
	}

	if info.Mode().Perm()&^MaxFileMode != 0 {
		return nil, fmt.Errorf("config %s has insecure mode %o; expected 0600 or stricter", expanded, info.Mode().Perm())
	}

	raw, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var file File
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	resolved.SourcePath = expanded
	endpoint := file.Endpoints[DefaultEndpointName]
	if resolved.BaseURL == "" {
		resolved.BaseURL = endpoint.BaseURL
	}
	if resolved.APIKey == "" {
		resolved.APIKey = endpoint.APIKey
	}
	if resolved.UserID == "" {
		resolved.UserID = file.Defaults.UserID
	}
	if resolved.AppID == "" {
		resolved.AppID = file.Defaults.AppID
	}
	if file.Timeouts.Request != "" {
		if d, err := time.ParseDuration(file.Timeouts.Request); err == nil {
			// Env wins only when explicitly set; if env not set, use file.
			if os.Getenv("MEM0_TIMEOUT") == "" {
				resolved.Timeout = d
			}
		}
	}
	if file.LogLevel != "" && os.Getenv("LOG_LEVEL") == "" {
		resolved.LogLevel = file.LogLevel
	}

	return resolved, nil
}

// Validate reports any missing required fields needed for HTTP calls.
func (r *Resolved) Validate() error {
	if strings.TrimSpace(r.BaseURL) == "" {
		return fmt.Errorf("base_url is required (set endpoints.default.base_url or MEM0_BASE_URL)")
	}
	return nil
}

func expandPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~"))
	}
	return trimmed, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseTimeoutOrDefault(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	return fallback
}
