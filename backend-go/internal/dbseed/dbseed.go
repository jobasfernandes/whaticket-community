package dbseed

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/setting"
	"github.com/jobasfernandes/whaticket-go-backend/internal/user"
)

type Options struct {
	AdminEmail    string
	AdminName     string
	AdminPassword string
}

type Result struct {
	AdminCreated    bool
	AdminID         uint
	SettingsApplied int
}

func (o Options) withDefaults() Options {
	if o.AdminEmail == "" {
		o.AdminEmail = "admin@whaticket.com"
	}
	if o.AdminName == "" {
		o.AdminName = "Admin"
	}
	if o.AdminPassword == "" {
		o.AdminPassword = "admin"
	}
	return o
}

func Run(ctx context.Context, db *gorm.DB, opts Options, logger *slog.Logger) (Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	opts = opts.withDefaults()
	res := Result{}
	created, id, err := ensureAdmin(ctx, db, opts, logger)
	if err != nil {
		return res, fmt.Errorf("seed admin: %w", err)
	}
	res.AdminCreated = created
	res.AdminID = id
	count, err := ensureSettings(ctx, db, logger)
	if err != nil {
		return res, fmt.Errorf("seed settings: %w", err)
	}
	res.SettingsApplied = count
	return res, nil
}

func ensureAdmin(ctx context.Context, db *gorm.DB, opts Options, logger *slog.Logger) (bool, uint, error) {
	var existing user.User
	err := db.WithContext(ctx).Where("email = ?", opts.AdminEmail).First(&existing).Error
	if err == nil {
		logger.Info("admin already present", "email", opts.AdminEmail, "id", existing.ID)
		return false, existing.ID, nil
	}
	if !stdErrors.Is(err, gorm.ErrRecordNotFound) {
		return false, 0, err
	}
	admin := user.User{
		Name:     opts.AdminName,
		Email:    opts.AdminEmail,
		Password: opts.AdminPassword,
		Profile:  "admin",
	}
	if err := db.WithContext(ctx).Create(&admin).Error; err != nil {
		return false, 0, err
	}
	logger.Info("admin created", "email", opts.AdminEmail, "id", admin.ID)
	return true, admin.ID, nil
}

func ensureSettings(ctx context.Context, db *gorm.DB, logger *slog.Logger) (int, error) {
	deps := &setting.Deps{DB: db}
	applied := 0
	if changed, err := upsertSetting(ctx, deps, "userCreation", "enabled"); err != nil {
		return applied, err
	} else if changed {
		applied++
	}
	logger.Info("settings ensured", "applied", applied)
	return applied, nil
}

func upsertSetting(ctx context.Context, deps *setting.Deps, key, value string) (bool, error) {
	if _, err := deps.Update(ctx, key, value); err != nil {
		if err.Status >= http.StatusInternalServerError {
			return false, err
		}
		return false, nil
	}
	return true, nil
}
