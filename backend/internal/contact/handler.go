package contact

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
	"github.com/canove/whaticket-community/backend/internal/platform/httpx"
)

type Handler struct {
	Deps   *Deps
	Logger *slog.Logger
}

type listResponse struct {
	Contacts []ContactDTO `json:"contacts"`
	Count    int64        `json:"count"`
	HasMore  bool         `json:"hasMore"`
}

type deleteResponse struct {
	Message string `json:"message"`
}

func (h *Handler) Routes(r chi.Router, accessSecret []byte) {
	r.Group(func(gr chi.Router) {
		gr.Use(auth.IsAuth(accessSecret))
		gr.Get("/contacts", httpx.Wrap(h.index))
		gr.Get("/contacts/{contactId}", httpx.Wrap(h.show))
		gr.Post("/contacts", httpx.Wrap(h.store))
		gr.Post("/contact", httpx.Wrap(h.getOrCreate))
		gr.Post("/contacts/import", httpx.Wrap(h.importPhoneContacts))
		gr.Put("/contacts/{contactId}", httpx.Wrap(h.update))
		gr.With(auth.IsAdmin).Delete("/contacts/{contactId}", httpx.Wrap(h.remove))
	})
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	searchParam := q.Get("searchParam")
	pageNumber, _ := strconv.Atoi(q.Get("pageNumber"))
	items, count, hasMore, appErr := h.Deps.List(r.Context(), searchParam, pageNumber)
	if appErr != nil {
		return appErr
	}
	dtos := make([]ContactDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, Serialize(&items[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Contacts: dtos, Count: count, HasMore: hasMore})
	return nil
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseContactID(r)
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
	id, appErr := parseContactID(r)
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
	id, appErr := parseContactID(r)
	if appErr != nil {
		return appErr
	}
	if err := h.Deps.Delete(r.Context(), id); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, deleteResponse{Message: "Contact deleted"})
	return nil
}

type getOrCreateRequest struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

func (h *Handler) getOrCreate(w http.ResponseWriter, r *http.Request) error {
	var req getOrCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	number := strings.TrimSpace(req.Number)
	if number == "" {
		return errors.New(errInvalidNumber, http.StatusBadRequest)
	}
	entity, appErr := h.Deps.CreateOrUpdate(r.Context(), CreateOrUpdateRequest{
		Number: number,
		Name:   strings.TrimSpace(req.Name),
	})
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(entity))
	return nil
}

func (h *Handler) importPhoneContacts(w http.ResponseWriter, _ *http.Request) error {
	httpx.WriteJSON(w, http.StatusNotImplemented, map[string]string{"message": "Phone contacts import not implemented"})
	return nil
}

func parseContactID(r *http.Request) (uint, *errors.AppError) {
	raw := chi.URLParam(r, "contactId")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	return uint(parsed), nil
}
