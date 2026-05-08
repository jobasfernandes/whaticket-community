package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/canove/whaticket-community/backend/internal/dbmigrate"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	dsn := firstNonEmpty(os.Getenv("DATABASE_URL"), os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		logger.Error("DATABASE_URL or POSTGRES_DSN is required")
		os.Exit(1)
	}

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	if err := Run(context.Background(), dsn, cmd, logger); err != nil {
		logger.Error("migration failed", "cmd", cmd, "err", err)
		os.Exit(1)
	}
	logger.Info("migration completed", "cmd", cmd)
}

func Run(ctx context.Context, dsn, cmd string, logger *slog.Logger) error {
	return dbmigrate.Run(ctx, dsn, cmd, logger)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
