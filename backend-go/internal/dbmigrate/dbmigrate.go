package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/jobasfernandes/whaticket-go-backend/migrations"
)

type Result struct {
	Before  int64
	After   int64
	Changed bool
}

func Up(ctx context.Context, dsn string, logger *slog.Logger) (Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return Result{}, fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Warn("close database", "err", cerr)
		}
	}()
	return UpDB(ctx, db, logger)
}

func UpDB(ctx context.Context, db *sql.DB, logger *slog.Logger) (Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return Result{}, fmt.Errorf("set dialect: %w", err)
	}
	before, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return Result{}, fmt.Errorf("read version: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return Result{}, fmt.Errorf("apply migrations: %w", err)
	}
	after, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return Result{}, fmt.Errorf("read version: %w", err)
	}
	res := Result{Before: before, After: after, Changed: after != before}
	if res.Changed {
		logger.Info("migrations applied", "from", before, "to", after)
	} else {
		logger.Info("database already up to date", "version", after)
	}
	return res, nil
}

func Run(ctx context.Context, dsn, cmd string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
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
		_, err := UpDB(ctx, db, logger)
		return err
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
