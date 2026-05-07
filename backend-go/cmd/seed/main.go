package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/dbseed"
)

func main() {
	email := flag.String("email", envOrDefault("SEED_ADMIN_EMAIL", "admin@whaticket.com"), "admin email")
	name := flag.String("name", envOrDefault("SEED_ADMIN_NAME", "Admin"), "admin name")
	password := flag.String("password", envOrDefault("SEED_ADMIN_PASSWORD", "admin"), "admin password")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		logger.Error("DATABASE_URL or POSTGRES_DSN is required")
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("db connect failed", "err", err)
		os.Exit(1)
	}

	if _, err := dbseed.Run(context.Background(), db, dbseed.Options{
		AdminEmail:    *email,
		AdminName:     *name,
		AdminPassword: *password,
	}, logger); err != nil {
		logger.Error("seed failed", "err", err)
		os.Exit(1)
	}
	logger.Info("seed complete")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
