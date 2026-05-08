package auth

import (
	"context"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const refreshCookieName = "jrt"

type UserRecord struct {
	ID           uint
	Name         string
	Email        string
	PasswordHash string
	Profile      string
	TokenVersion int
}

type SerializedUser struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Profile  string `json:"profile"`
	Queues   []any  `json:"queues"`
	Whatsapp any    `json:"whatsapp"`
}

type UserLoader interface {
	FindByEmail(ctx context.Context, db *gorm.DB, email string) (*UserRecord, error)
	FindByID(ctx context.Context, db *gorm.DB, id uint) (*UserRecord, error)
	Serialize(ctx context.Context, db *gorm.DB, user *UserRecord) (SerializedUser, error)
}

type SettingChecker interface {
	FindAPIToken(ctx context.Context, db *gorm.DB, token string) (bool, error)
}

func Login(ctx context.Context, db *gorm.DB, loader UserLoader, accessSecret, refreshSecret []byte, email, password string) (string, string, *UserRecord, *errors.AppError) {
	user, err := loader.FindByEmail(ctx, db, email)
	if err != nil || user == nil {
		return "", "", nil, errors.New("ERR_INVALID_CREDENTIALS", http.StatusUnauthorized)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", "", nil, errors.New("ERR_INVALID_CREDENTIALS", http.StatusUnauthorized)
	}
	access, err := CreateAccessToken(UserSummary{ID: user.ID, Name: user.Name, Profile: user.Profile}, accessSecret)
	if err != nil {
		return "", "", nil, errors.Wrap(err, "ERR_TOKEN_SIGN", http.StatusInternalServerError)
	}
	refresh, err := CreateRefreshToken(user.ID, user.TokenVersion, refreshSecret)
	if err != nil {
		return "", "", nil, errors.Wrap(err, "ERR_TOKEN_SIGN", http.StatusInternalServerError)
	}
	return access, refresh, user, nil
}

func Refresh(ctx context.Context, db *gorm.DB, loader UserLoader, accessSecret, refreshSecret []byte, refreshTokenStr string) (string, string, *UserRecord, *errors.AppError) {
	if refreshTokenStr == "" {
		return "", "", nil, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	claims, err := ParseRefreshToken(refreshTokenStr, refreshSecret)
	if err != nil {
		return "", "", nil, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	user, err := loader.FindByID(ctx, db, claims.ID)
	if err != nil || user == nil {
		return "", "", nil, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	if user.TokenVersion != claims.TokenVersion {
		return "", "", nil, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	access, err := CreateAccessToken(UserSummary{ID: user.ID, Name: user.Name, Profile: user.Profile}, accessSecret)
	if err != nil {
		return "", "", nil, errors.Wrap(err, "ERR_TOKEN_SIGN", http.StatusInternalServerError)
	}
	rotated, err := CreateRefreshToken(user.ID, user.TokenVersion, refreshSecret)
	if err != nil {
		return "", "", nil, errors.Wrap(err, "ERR_TOKEN_SIGN", http.StatusInternalServerError)
	}
	return access, rotated, user, nil
}

func setRefreshCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(refreshTokenTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isProduction(),
	}
	http.SetCookie(w, cookie)
}

func clearRefreshCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isProduction(),
	}
	http.SetCookie(w, cookie)
}

func isProduction() bool {
	return os.Getenv("APP_ENV") == "production"
}
