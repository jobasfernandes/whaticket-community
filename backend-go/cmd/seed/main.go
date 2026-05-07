package main

import (
	"context"
	stdErrors "errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/setting"
	"github.com/jobasfernandes/whaticket-go-backend/internal/user"
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

	ctx := context.Background()

	if err := ensureAdmin(ctx, db, *email, *name, *password, logger); err != nil {
		logger.Error("seed admin failed", "err", err)
		os.Exit(1)
	}
	if err := ensureSettings(ctx, db, logger); err != nil {
		logger.Error("seed settings failed", "err", err)
		os.Exit(1)
	}
	logger.Info("seed complete")
}

func ensureAdmin(ctx context.Context, db *gorm.DB, email, name, password string, logger *slog.Logger) error {
	var existing user.User
	err := db.WithContext(ctx).Where("email = ?", email).First(&existing).Error
	if err == nil {
		logger.Info("admin already exists; skipping", "email", email, "id", existing.ID)
		return nil
	}
	if !stdErrors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	admin := user.User{
		Name:     name,
		Email:    email,
		Password: password,
		Profile:  "admin",
	}
	if err := db.WithContext(ctx).Create(&admin).Error; err != nil {
		return err
	}
	logger.Info("admin created", "email", email, "id", admin.ID)
	return nil
}

func ensureSettings(ctx context.Context, db *gorm.DB, logger *slog.Logger) error {
	deps := &setting.Deps{DB: db}
	if err := upsertSetting(ctx, deps, "userCreation", "enabled"); err != nil {
		return err
	}
	logger.Info("settings ensured")
	return nil
}

func upsertSetting(ctx context.Context, deps *setting.Deps, key, value string) error {
	if _, err := deps.Update(ctx, key, value); err != nil {
		if err.Status >= http.StatusInternalServerError {
			return err
		}
		return nil
	}
	return nil
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
