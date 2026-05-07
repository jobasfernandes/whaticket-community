package message

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type Handler struct {
	Deps         *Deps
	Logger       *slog.Logger
	AccessSecret []byte
}

type listResponse struct {
	Count    int64        `json:"count"`
	HasMore  bool         `json:"hasMore"`
	Messages []MessageDTO `json:"messages"`
	Ticket   any          `json:"ticket"`
}

type deleteResponse struct {
	Message string `json:"message"`
}

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(gr chi.Router) {
		gr.Use(auth.IsAuth(h.AccessSecret))
		gr.Get("/messages/{ticketId}", httpx.Wrap(h.list))
		gr.Delete("/messages/{messageId}", httpx.Wrap(h.delete))
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	ticketID, appErr := parseTicketID(r)
	if appErr != nil {
		return appErr
	}
	pageNumber, _ := strconv.Atoi(r.URL.Query().Get("pageNumber"))
	actor := claimsPointer(r)
	ticket, appErr := h.Deps.TicketSvc.Show(r.Context(), ticketID, actor)
	if appErr != nil {
		return appErr
	}
	messages, count, hasMore, appErr := h.Deps.List(r.Context(), ticketID, pageNumber)
	if appErr != nil {
		return appErr
	}
	dtos := make([]MessageDTO, 0, len(messages))
	for i := range messages {
		dtos = append(dtos, Serialize(&messages[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{
		Count:    count,
		HasMore:  hasMore,
		Messages: dtos,
		Ticket:   h.Deps.TicketSvc.SerializeTicket(ticket),
	})
	return nil
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) error {
	messageID := chi.URLParam(r, "messageId")
	if messageID == "" {
		return errors.New(errBadRequest, http.StatusBadRequest)
	}
	actor := claimsPointer(r)
	if actor == nil {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	if appErr := h.Deps.Delete(r.Context(), messageID, actor); appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, deleteResponse{Message: "Message deleted"})
	return nil
}

func parseTicketID(r *http.Request) (uint, *errors.AppError) {
	raw := chi.URLParam(r, "ticketId")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New(errBadRequest, http.StatusBadRequest)
	}
	return uint(parsed), nil
}

func claimsPointer(r *http.Request) *auth.UserClaims {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		return nil
	}
	return &claims
}
