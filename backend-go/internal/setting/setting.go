package setting

import (
	"net/http"
	"time"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const (
	KeyUserCreation = "userCreation"
	KeyUserApiToken = "userApiToken"
)

const (
	apiTokenMinLength = 16
	apiTokenMaxLength = 128
)

var allowedKeys = map[string]bool{
	KeyUserCreation: true,
	KeyUserApiToken: true,
}

var sensitiveKeys = map[string]bool{
	KeyUserApiToken: true,
}

type Setting struct {
	Key       string `gorm:"primaryKey;size:50"`
	Value     string `gorm:"type:text;not null;default:''"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Setting) TableName() string {
	return "settings"
}

type SettingDTO struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func Serialize(s *Setting) SettingDTO {
	return SettingDTO{
		Key:       s.Key,
		Value:     s.Value,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

type UpdateRequest struct {
	Value string `json:"value" validate:"required"`
}

func validateValue(key, value string) *errors.AppError {
	if !allowedKeys[key] {
		return errors.New("ERR_UNKNOWN_SETTING_KEY", http.StatusBadRequest)
	}
	switch key {
	case KeyUserCreation:
		if value != "enabled" && value != "disabled" {
			return errors.New("ERR_INVALID_SETTING_VALUE", http.StatusBadRequest)
		}
	case KeyUserApiToken:
		if len(value) < apiTokenMinLength || len(value) > apiTokenMaxLength {
			return errors.New("ERR_INVALID_SETTING_VALUE", http.StatusBadRequest)
		}
	}
	return nil
}
