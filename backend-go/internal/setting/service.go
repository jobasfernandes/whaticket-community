package setting

import (
	"context"
	stdErrors "errors"
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type Deps struct {
	DB *gorm.DB
	WS WSPublisher
}

const (
	wsChannelNotification = "notification"
	wsEventSettingsUpdate = "settings.update"
	wsActionUpdate        = "update"
)

func (d *Deps) ListSettings(ctx context.Context) ([]Setting, *errors.AppError) {
	var settings []Setting
	if err := d.DB.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, errors.Wrap(err, "ERR_LIST_SETTINGS", http.StatusInternalServerError)
	}
	return settings, nil
}

func (d *Deps) Update(ctx context.Context, key, value string) (*Setting, *errors.AppError) {
	if err := validateValue(key, value); err != nil {
		return nil, err
	}

	var s Setting
	if err := d.DB.WithContext(ctx).First(&s, "key = ?", key).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("ERR_NO_SETTING_FOUND", http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_UPDATE_SETTING", http.StatusInternalServerError)
	}

	if err := d.DB.WithContext(ctx).Model(&s).Update("value", value).Error; err != nil {
		return nil, errors.Wrap(err, "ERR_UPDATE_SETTING", http.StatusInternalServerError)
	}

	if sensitiveKeys[key] {
		slog.Info("setting updated", "key", key, "value", "***")
	} else {
		slog.Info("setting updated", "key", key, "value", value)
		if d.WS != nil {
			d.WS.Publish(wsChannelNotification, wsEventSettingsUpdate, map[string]any{
				"action":  wsActionUpdate,
				"setting": Serialize(&s),
			})
		}
	}

	return &s, nil
}

type Checker struct {
	db *gorm.DB
}

func NewSettingChecker(db *gorm.DB) *Checker {
	return &Checker{db: db}
}

func (c *Checker) Check(ctx context.Context, db *gorm.DB, key string) (string, *errors.AppError) {
	target := db
	if target == nil {
		target = c.db
	}
	return Check(ctx, target, key)
}

func (c *Checker) FindAPIToken(ctx context.Context, db *gorm.DB, token string) (bool, error) {
	target := db
	if target == nil {
		target = c.db
	}
	if _, appErr := FindByValue(ctx, target, token); appErr != nil {
		if appErr.Code == "ERR_NO_SETTING_FOUND" {
			return false, nil
		}
		return false, appErr
	}
	return true, nil
}

func Check(ctx context.Context, db *gorm.DB, key string) (string, *errors.AppError) {
	var s Setting
	if err := db.WithContext(ctx).First(&s, "key = ?", key).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("ERR_NO_SETTING_FOUND", http.StatusNotFound)
		}
		return "", errors.Wrap(err, "ERR_CHECK_SETTING", http.StatusInternalServerError)
	}
	return s.Value, nil
}

func FindByValue(ctx context.Context, db *gorm.DB, value string) (*Setting, *errors.AppError) {
	if value == "" {
		return nil, errors.New("ERR_NO_SETTING_FOUND", http.StatusNotFound)
	}
	var s Setting
	if err := db.WithContext(ctx).Where("key = ? AND value = ?", KeyUserApiToken, value).First(&s).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("ERR_NO_SETTING_FOUND", http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_FIND_SETTING_BY_VALUE", http.StatusInternalServerError)
	}
	return &s, nil
}
