package auth

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	apperr "github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type Deps struct {
	DB            *gorm.DB
	Loader        UserLoader
	AccessSecret  []byte
	RefreshSecret []byte
}

func (d *Deps) Routes(r chi.Router) {
	r.Post("/login", httpx.Wrap(d.handleLogin))
	r.Post("/refresh_token", httpx.Wrap(d.handleRefresh))
	r.With(IsAuth(d.AccessSecret)).Delete("/logout", httpx.Wrap(d.handleLogout))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sessionResponse struct {
	Token string         `json:"token"`
	User  SerializedUser `json:"user"`
}

func (d *Deps) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	access, refresh, user, appErr := Login(r.Context(), d.DB, d.Loader, d.AccessSecret, d.RefreshSecret, req.Email, req.Password)
	if appErr != nil {
		return appErr
	}
	serialized, sErr := d.Loader.Serialize(r.Context(), d.DB, user)
	if sErr != nil {
		return apperr.Wrap(sErr, "ERR_SERIALIZE_USER", http.StatusInternalServerError)
	}
	setRefreshCookie(w, refresh)
	httpx.WriteJSON(w, http.StatusOK, sessionResponse{Token: access, User: serialized})
	return nil
}

func (d *Deps) handleRefresh(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		return apperr.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	access, refresh, user, appErr := Refresh(r.Context(), d.DB, d.Loader, d.AccessSecret, d.RefreshSecret, cookie.Value)
	if appErr != nil {
		clearRefreshCookie(w)
		return appErr
	}
	serialized, sErr := d.Loader.Serialize(r.Context(), d.DB, user)
	if sErr != nil {
		return apperr.Wrap(sErr, "ERR_SERIALIZE_USER", http.StatusInternalServerError)
	}
	setRefreshCookie(w, refresh)
	httpx.WriteJSON(w, http.StatusOK, sessionResponse{Token: access, User: serialized})
	return nil
}

func (d *Deps) handleLogout(w http.ResponseWriter, r *http.Request) error {
	clearRefreshCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{})
	return nil
}
