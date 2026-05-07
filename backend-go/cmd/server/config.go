package main

import (
	"fmt"
	"os"
	"strings"
)

type appConfig struct {
	ListenAddr    string
	DatabaseDSN   string
	RMQURL        string
	FrontendURL   string
	AccessSecret  string
	RefreshSecret string
	LogLevel      string
	AutoMigrate   bool
	AutoSeed      bool
	SeedEmail     string
	SeedName      string
	SeedPassword  string
}

func loadConfig() (appConfig, error) {
	cfg := appConfig{
		ListenAddr:    envOrDefault("BACKEND_LISTEN_ADDR", ":8080"),
		DatabaseDSN:   firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("POSTGRES_DSN")),
		RMQURL:        envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		FrontendURL:   envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		AccessSecret:  os.Getenv("JWT_SECRET"),
		RefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		LogLevel:      envOrDefault("LOG_LEVEL", "info"),
		AutoMigrate:   envBool("AUTO_MIGRATE", false),
		AutoSeed:      envBool("AUTO_SEED", false),
		SeedEmail:     envOrDefault("SEED_ADMIN_EMAIL", "admin@whaticket.com"),
		SeedName:      envOrDefault("SEED_ADMIN_NAME", "Admin"),
		SeedPassword:  envOrDefault("SEED_ADMIN_PASSWORD", "admin"),
	}
	if cfg.DatabaseDSN == "" {
		return cfg, fmt.Errorf("DATABASE_URL or POSTGRES_DSN is required")
	}
	if cfg.AccessSecret == "" {
		cfg.AccessSecret = "test-access-secret-development-only"
	}
	if cfg.RefreshSecret == "" {
		cfg.RefreshSecret = "test-refresh-secret-development-only"
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func allowedOrigins(raw string) []string {
	if raw == "" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimRight(strings.TrimSpace(p), "/")
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
