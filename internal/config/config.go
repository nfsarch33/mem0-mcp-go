package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL   string
	APIKey    string
	UserID    string
	AppID     string
	Transport string
	SSEAddr   string
	Timeout   time.Duration
	LogLevel  string

	DualWrite    bool
	CloudURL     string
	CloudAPIKey  string
	ReadSource   string
	BackupURL    string
	BackupAPIKey string
}

// Load reads MEM0_* environment variables and returns a Config with
// generic defaults. Operators MUST set MEM0_BASE_URL in their deploy
// environment; the package no longer pre-wires loopback or personal
// tunnel topology. UserID and AppID default to neutral placeholders so
// the package is publishable as a true OSS Mem0 MCP server. Older MCP
// client configs that still use MEM0_DEFAULT_USER_ID /
// MEM0_DEFAULT_APP_ID remain supported as compatibility fallbacks.
func Load() Config {
	return Config{
		BaseURL:   getenv("MEM0_BASE_URL", ""),
		APIKey:    os.Getenv("MEM0_API_KEY"),
		UserID:    getenv("MEM0_USER_ID", getenv("MEM0_DEFAULT_USER_ID", "default-user")),
		AppID:     getenv("MEM0_APP_ID", getenv("MEM0_DEFAULT_APP_ID", "default-app")),
		Transport: getenv("MCP_TRANSPORT", "stdio"),
		SSEAddr:   getenv("MCP_SSE_ADDR", ":9092"),
		Timeout:   getenvDuration("MEM0_TIMEOUT", 120*time.Second),
		LogLevel:  getenv("LOG_LEVEL", "info"),

		DualWrite:    getenvBool("MEM0_DUAL_WRITE", false),
		CloudURL:     getenv("MEM0_CLOUD_URL", "https://api.mem0.ai"),
		CloudAPIKey:  os.Getenv("MEM0_CLOUD_API_KEY"),
		ReadSource:   getenv("MEM0_READ_SOURCE", "oss"),
		BackupURL:    os.Getenv("MEM0_BACKUP_URL"),
		BackupAPIKey: os.Getenv("MEM0_BACKUP_API_KEY"),
	}
}

// DualWriteEnabled returns true when dual-write is on and at least the
// cloud target has a URL and key configured.
func (c Config) DualWriteEnabled() bool {
	return c.DualWrite && c.CloudURL != "" && c.CloudAPIKey != ""
}

// BackupEnabled returns true when a backup target is fully configured.
func (c Config) BackupEnabled() bool {
	return c.BackupURL != "" && c.BackupAPIKey != ""
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
