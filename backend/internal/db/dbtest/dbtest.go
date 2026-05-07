//go:build integration

package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/canove/whaticket-community/backend/migrations"
)

const (
	postgresImage    = "postgres:15-alpine"
	postgresDatabase = "whaticket_test"
	postgresUser     = "whaticket"
	postgresPass     = "whaticket"
	defaultTimeout   = 90 * time.Second
)

type Postgres struct {
	Container *tcpostgres.PostgresContainer
	DSN       string
	DB        *gorm.DB
	SQL       *sql.DB
}

func StartPostgres(ctx context.Context, t *testing.T) *Postgres {
	return startPostgres(ctx, t, true)
}

func StartPostgresRaw(ctx context.Context, t *testing.T) *Postgres {
	return startPostgres(ctx, t, false)
}

func startPostgres(ctx context.Context, t *testing.T, runMigrations bool) *Postgres {
	t.Helper()

	startCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	container, err := tcpostgres.Run(
		startCtx,
		postgresImage,
		tcpostgres.WithDatabase(postgresDatabase),
		tcpostgres.WithUsername(postgresUser),
		tcpostgres.WithPassword(postgresPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(defaultTimeout),
		),
	)
	if err != nil {
		t.Skipf("postgres testcontainer unavailable: %v", err)
		return nil
	}

	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	dsn, err := container.ConnectionString(startCtx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}

	if runMigrations {
		if err := applyMigrations(startCtx, dsn); err != nil {
			t.Fatalf("apply migrations: %v", err)
		}
	}

	gormDB, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(2)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return &Postgres{
		Container: container,
		DSN:       dsn,
		DB:        gormDB,
		SQL:       sqlDB,
	}
}

func applyMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func (p *Postgres) Truncate(t *testing.T, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return
	}
	stmt := "TRUNCATE TABLE "
	for i, table := range tables {
		if i > 0 {
			stmt += ", "
		}
		stmt += table
	}
	stmt += " RESTART IDENTITY CASCADE"
	if err := p.DB.Exec(stmt).Error; err != nil {
		t.Fatalf("truncate %v: %v", tables, err)
	}
}
