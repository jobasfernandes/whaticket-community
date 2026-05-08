package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	RMQUrl       string
	S3Endpoint   string
	S3AccessKey  string
	S3SecretKey  string
	S3Bucket     string
	S3PublicURL  string
	S3UseSSL     bool
	SQLStorePath string
	PlatformType string
	OSName       string
	HealthPort   string
	LogLevel     string
	LogFormat    string
}

func Load() (*Config, error) {
	cfg := &Config{
		RMQUrl:       getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		S3Endpoint:   getEnv("WORKER_S3_ENDPOINT", ""),
		S3AccessKey:  getEnv("WORKER_S3_ACCESS_KEY", ""),
		S3SecretKey:  getEnv("WORKER_S3_SECRET_KEY", ""),
		S3Bucket:     getEnv("WORKER_S3_BUCKET", "whaticket-media"),
		S3PublicURL:  getEnv("WORKER_S3_PUBLIC_URL", ""),
		S3UseSSL:     getEnvBool("WORKER_S3_USE_SSL", false),
		SQLStorePath: getEnv("WORKER_SQLSTORE_PATH", "./worker-data/whatsmeow.db"),
		PlatformType: getEnv("WORKER_PLATFORM_TYPE", "DESKTOP"),
		OSName:       getEnv("WORKER_OS_NAME", "whaticket-worker"),
		HealthPort:   getEnv("WORKER_HEALTH_PORT", "8081"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		LogFormat:    getEnv("LOG_FORMAT", "json"),
	}

	if strings.TrimSpace(cfg.RMQUrl) == "" {
		return nil, errors.New("config: RABBITMQ_URL is required")
	}
	return cfg, nil
}

func (c *Config) Redacted() map[string]string {
	return map[string]string{
		"rmq_url":       redactURL(c.RMQUrl),
		"s3_endpoint":   c.S3Endpoint,
		"s3_access_key": maskString(c.S3AccessKey),
		"s3_secret_key": maskString(c.S3SecretKey),
		"s3_bucket":     c.S3Bucket,
		"s3_public_url": c.S3PublicURL,
		"s3_use_ssl":    strconv.FormatBool(c.S3UseSSL),
		"sqlstore_path": c.SQLStorePath,
		"platform_type": c.PlatformType,
		"os_name":       c.OSName,
		"health_port":   c.HealthPort,
		"log_level":     c.LogLevel,
		"log_format":    c.LogFormat,
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return parsed
}

func redactURL(raw string) string {
	at := strings.LastIndex(raw, "@")
	if at < 0 {
		return raw
	}
	scheme := strings.Index(raw, "://")
	if scheme < 0 || scheme >= at {
		return raw
	}
	return raw[:scheme+3] + "***:***" + raw[at:]
}

func maskString(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "***"
	}
	return v[:2] + "***" + v[len(v)-2:]
}
