package main

import (
	"fmt"
	"os"
)

type appConfig struct {
	ListenAddr    string
	DatabaseDSN   string
	RMQURL        string
	FrontendURL   string
	AccessSecret  string
	RefreshSecret string
	LogLevel      string
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
