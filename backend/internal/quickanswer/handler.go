package quickanswer

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
	"github.com/canove/whaticket-community/backend/internal/platform/httpx"
)

type Handler struct {
	Deps *Deps
}

type listResponse struct {
	QuickAnswers []QuickAnswerDTO `json:"quickAnswers"`
	Count        int64            `json:"count"`
	HasMore      bool             `json:"hasMore"`
}

type deleteResponse struct {
	Message string `json:"message"`
}

func (h *Handler) Routes(r chi.Router, accessSecret []byte) {
	r.Group(func(gr chi.Router) {
		gr.Use(auth.IsAuth(accessSecret))
		gr.Get("/quickAnswers", httpx.Wrap(h.index))
		gr.Post("/quickAnswers", httpx.Wrap(h.store))
		gr.Get("/quickAnswers/{id}", httpx.Wrap(h.show))
		gr.Put("/quickAnswers/{id}", httpx.Wrap(h.update))
		gr.Delete("/quickAnswers/{id}", httpx.Wrap(h.remove))
	})
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	searchParam := r.URL.Query().Get("searchParam")
	pageNumber := parsePageNumber(r.URL.Query().Get("pageNumber"))
	items, count, hasMore, appErr := h.Deps.List(r.Context(), searchParam, pageNumber)
	if appErr != nil {
		return appErr
	}
	dtos := make([]QuickAnswerDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, Serialize(&items[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{QuickAnswers: dtos, Count: count, HasMore: hasMore})
	return nil
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) error {
	id, err := parseIDParam(r)
	if err != nil {
		return err
	}
	entity, appErr := h.Deps.Show(r.Context(), id)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(entity))
	return nil
}

func (h *Handler) store(w http.ResponseWriter, r *http.Request) error {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	entity, appErr := h.Deps.Create(r.Context(), req)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusCreated, Serialize(entity))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := parseIDParam(r)
	if err != nil {
		return err
	}
	var req UpdateRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	entity, appErr := h.Deps.Update(r.Context(), id, req)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(entity))
	return nil
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) error {
	id, err := parseIDParam(r)
	if err != nil {
		return err
	}
	if appErr := h.Deps.Delete(r.Context(), id); appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, deleteResponse{Message: "Quick Answer deleted"})
	return nil
}

func parsePageNumber(raw string) int {
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

func parseIDParam(r *http.Request) (uint, *errors.AppError) {
	raw := chi.URLParam(r, "id")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	return uint(parsed), nil
}
