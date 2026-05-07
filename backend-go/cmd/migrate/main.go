package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/jobasfernandes/whaticket-go-backend/migrations"
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
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Warn("close database", "err", cerr)
		}
	}()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	switch cmd {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "down":
		return goose.DownContext(ctx, db, ".")
	case "status":
		return goose.StatusContext(ctx, db, ".")
	case "version":
		v, err := goose.GetDBVersionContext(ctx, db)
		if err != nil {
			return err
		}
		logger.Info("current version", "version", v)
		return nil
	case "redo":
		return goose.RedoContext(ctx, db, ".")
	case "reset":
		return goose.ResetContext(ctx, db, ".")
	default:
		return fmt.Errorf("unknown command %q (supported: up, down, status, version, redo, reset)", cmd)
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
