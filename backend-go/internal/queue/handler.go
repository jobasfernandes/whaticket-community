package queue

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type Handler struct {
	Deps *Deps
}

type deleteResponse struct {
	Message string `json:"message"`
}

func (h *Handler) Routes(r chi.Router, accessSecret []byte) {
	r.Group(func(gr chi.Router) {
		gr.Use(auth.IsAuth(accessSecret))
		gr.Use(auth.IsAdmin)
		gr.Get("/queue", httpx.Wrap(h.index))
		gr.Post("/queue", httpx.Wrap(h.store))
		gr.Get("/queue/{queueId}", httpx.Wrap(h.show))
		gr.Put("/queue/{queueId}", httpx.Wrap(h.update))
		gr.Delete("/queue/{queueId}", httpx.Wrap(h.remove))
	})
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	items, appErr := h.Deps.List(r.Context())
	if appErr != nil {
		return appErr
	}
	dtos := make([]QueueDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, Serialize(&items[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, dtos)
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

func (h *Handler) show(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseQueueID(r)
	if appErr != nil {
		return appErr
	}
	entity, appErr := h.Deps.Show(r.Context(), id)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(entity))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseQueueID(r)
	if appErr != nil {
		return appErr
	}
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	id, appErr := parseQueueID(r)
	if appErr != nil {
		return appErr
	}
	if err := h.Deps.Delete(r.Context(), id); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, deleteResponse{Message: "Queue deleted"})
	return nil
}

func parseQueueID(r *http.Request) (uint, *errors.AppError) {
	raw := chi.URLParam(r, "queueId")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	return uint(parsed), nil
}
