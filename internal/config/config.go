package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	BaseURL     string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    valueOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		BaseURL:     strings.TrimRight(valueOrDefault("BASE_URL", "http://localhost:8080"), "/"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || strings.Trim(base.Hostname(), ".") == "" {
		return Config{}, fmt.Errorf("BASE_URL must be a valid HTTP or HTTPS URL")
	}
	if base.User != nil || base.RawQuery != "" || base.ForceQuery || strings.Contains(cfg.BaseURL, "#") {
		return Config{}, fmt.Errorf("BASE_URL must be an HTTP or HTTPS URL without credentials, query or fragment")
	}
	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
