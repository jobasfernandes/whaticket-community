package user

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	apperr "github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type Handler struct {
	Deps *Deps
}

type listResponse struct {
	Users   []UserDTO `json:"users"`
	Count   int64     `json:"count"`
	HasMore bool      `json:"hasMore"`
}

func (h *Handler) Routes(r chi.Router, accessSecret []byte) {
	r.Group(func(g chi.Router) {
		g.Use(auth.IsAuth(accessSecret))
		g.Use(auth.IsAdmin)
		g.Get("/users", httpx.Wrap(h.index))
		g.Get("/users/{userId}", httpx.Wrap(h.show))
		g.Post("/users", httpx.Wrap(h.create))
		g.Put("/users/{userId}", httpx.Wrap(h.update))
		g.Delete("/users/{userId}", httpx.Wrap(h.remove))
	})
}

func (h *Handler) MountSignup(r chi.Router) {
	r.Post("/signup", httpx.Wrap(h.signup))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	searchParam := q.Get("searchParam")
	pageNumber, _ := strconv.Atoi(q.Get("pageNumber"))
	users, count, hasMore, err := h.Deps.List(r.Context(), searchParam, pageNumber)
	if err != nil {
		return err
	}
	dtos := make([]UserDTO, 0, len(users))
	for i := range users {
		dtos = append(dtos, Serialize(&users[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Users: dtos, Count: count, HasMore: hasMore})
	return nil
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) error {
	id, err := parseUserID(r)
	if err != nil {
		return err
	}
	u, appErr := h.Deps.Show(r.Context(), id)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(u))
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	u, appErr := h.Deps.Create(r.Context(), req, false, true)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusCreated, Serialize(u))
	return nil
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) error {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	u, appErr := h.Deps.Create(r.Context(), req, true, false)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusCreated, Serialize(u))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, err := parseUserID(r)
	if err != nil {
		return err
	}
	var req UpdateRequest
	if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil {
		return apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	u, appErr := h.Deps.Update(r.Context(), id, req, true)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(u))
	return nil
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) error {
	id, err := parseUserID(r)
	if err != nil {
		return err
	}
	if appErr := h.Deps.Delete(r.Context(), id, true); appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func parseUserID(r *http.Request) (uint, *apperr.AppError) {
	raw := chi.URLParam(r, "userId")
	if raw == "" {
		return 0, apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, apperr.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	return uint(v), nil
}
