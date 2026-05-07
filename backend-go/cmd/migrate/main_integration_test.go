//go:build integration

package main

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jobasfernandes/whaticket-go-backend/internal/db/dbtest"
)

const expectedMigrations = 11

func TestMigrateRunUpDownSmoke(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgresRaw(ctx, t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := Run(ctx, pg.DSN, "up", logger); err != nil {
		t.Fatalf("Run up: %v", err)
	}

	versions := readGooseVersions(t, pg.DSN)
	applied := 0
	for _, v := range versions {
		if v > 0 {
			applied++
		}
	}
	if applied != expectedMigrations {
		t.Fatalf("expected %d applied migrations, got %d (versions=%v)", expectedMigrations, applied, versions)
	}

	topBefore := maxVersion(versions)

	if err := Run(ctx, pg.DSN, "down", logger); err != nil {
		t.Fatalf("Run down: %v", err)
	}

	afterVersions := readGooseVersions(t, pg.DSN)
	topAfter := maxVersion(afterVersions)
	if topAfter == topBefore {
		t.Fatalf("expected top migration %d to be rolled back, still present in %v", topBefore, afterVersions)
	}
}

func readGooseVersions(t *testing.T, dsn string) []int64 {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query("SELECT version_id FROM goose_db_version WHERE is_applied = true ORDER BY version_id ASC")
	if err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iter: %v", err)
	}
	return out
}

func maxVersion(values []int64) int64 {
	var m int64
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}
