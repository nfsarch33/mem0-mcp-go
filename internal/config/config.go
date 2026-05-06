package config

import (
	"os"
	"strconv"
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
}

func Load() Config {
	return Config{
		BaseURL:   getenv("MEM0_BASE_URL", "http://127.0.0.1:18888"),
		APIKey:    os.Getenv("MEM0_API_KEY"),
		UserID:    getenv("MEM0_USER_ID", getenv("MEM0_DEFAULT_USER_ID", "nfsarch33")),
		AppID:     getenv("MEM0_APP_ID", "cursor-global-kb"),
		Transport: getenv("MCP_TRANSPORT", "stdio"),
		SSEAddr:   getenv("MCP_SSE_ADDR", ":9092"),
		Timeout:   getenvDuration("MEM0_TIMEOUT", 30*time.Second),
		LogLevel:  getenv("LOG_LEVEL", "info"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
