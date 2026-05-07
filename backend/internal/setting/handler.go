package setting

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/canove/whaticket-community/backend/internal/auth"
	apperr "github.com/canove/whaticket-community/backend/internal/platform/errors"
	"github.com/canove/whaticket-community/backend/internal/platform/httpx"
)

type Handler struct {
	Deps *Deps
}

func (h *Handler) Routes(r chi.Router, accessSecret []byte) {
	r.With(auth.IsAuth(accessSecret), auth.IsAdmin).Get("/settings", httpx.Wrap(h.index))
	r.With(auth.IsAuth(accessSecret), auth.IsAdmin).Put("/settings/{settingKey}", httpx.Wrap(h.update))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	settings, err := h.Deps.ListSettings(r.Context())
	if err != nil {
		return err
	}
	dtos := make([]SettingDTO, 0, len(settings))
	for i := range settings {
		dtos = append(dtos, Serialize(&settings[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, dtos)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	key := chi.URLParam(r, "settingKey")
	if key == "" {
		return apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}

	updated, err := h.Deps.Update(r.Context(), key, req.Value)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(updated))
	return nil
}
