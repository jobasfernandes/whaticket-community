package auth

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type ctxKey int

const userCtxKey ctxKey = iota

type UserClaims struct {
	ID      uint
	Profile string
}

func ContextWithUser(ctx context.Context, claims UserClaims) context.Context {
	return context.WithValue(ctx, userCtxKey, claims)
}

func UserFromContext(ctx context.Context) (UserClaims, bool) {
	claims, ok := ctx.Value(userCtxKey).(UserClaims)
	return claims, ok
}

func IsAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				httpx.WriteError(w, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized))
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
				httpx.WriteError(w, errors.New("ERR_INVALID_TOKEN", http.StatusUnauthorized))
				return
			}
			claims, err := ParseAccessToken(parts[1], secret)
			if err != nil {
				httpx.WriteError(w, errors.New("ERR_INVALID_TOKEN", http.StatusUnauthorized))
				return
			}
			ctx := ContextWithUser(r.Context(), UserClaims{ID: claims.ID, Profile: claims.Profile})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func IsAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := UserFromContext(r.Context())
		if !ok || claims.Profile != "admin" {
			httpx.WriteError(w, errors.New("ERR_NO_PERMISSION", http.StatusForbidden))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func IsAuthAPI(db *gorm.DB, settings SettingChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				httpx.WriteError(w, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized))
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
				httpx.WriteError(w, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized))
				return
			}
			ok, err := settings.FindAPIToken(r.Context(), db, parts[1])
			if err != nil || !ok {
				httpx.WriteError(w, errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
