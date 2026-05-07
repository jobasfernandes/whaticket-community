package main

import (
	"context"
	"database/sql"
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

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		logger.Error("open database", "err", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Warn("close database", "err", cerr)
		}
	}()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("set dialect", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	switch cmd {
	case "up":
		err = goose.UpContext(ctx, db, ".")
	case "down":
		err = goose.DownContext(ctx, db, ".")
	case "status":
		err = goose.StatusContext(ctx, db, ".")
	case "version":
		var v int64
		v, err = goose.GetDBVersionContext(ctx, db)
		if err == nil {
			logger.Info("current version", "version", v)
		}
	case "redo":
		err = goose.RedoContext(ctx, db, ".")
	case "reset":
		err = goose.ResetContext(ctx, db, ".")
	default:
		logger.Error("unknown command", "cmd", cmd, "supported", []string{"up", "down", "status", "version", "redo", "reset"})
		os.Exit(2)
	}

	if err != nil {
		logger.Error("migration failed", "cmd", cmd, "err", err)
		os.Exit(1)
	}

	logger.Info("migration completed", "cmd", cmd)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
