package ticket

import (
	"encoding/json"
	stdErrors "errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/contact"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type Handler struct {
	Deps         *Deps
	Logger       *slog.Logger
	AccessSecret []byte
}

type listResponse struct {
	Tickets []TicketDTO `json:"tickets"`
	Count   int64       `json:"count"`
	HasMore bool        `json:"hasMore"`
}

type updateRequest struct {
	Status  *string `json:"status,omitempty"`
	UserID  **uint  `json:"userId,omitempty"`
	QueueID **uint  `json:"queueId,omitempty"`
}

type storeRequest struct {
	ContactID  uint    `json:"contactId"`
	WhatsappID *uint   `json:"whatsappId,omitempty"`
	Status     *string `json:"status,omitempty"`
	UserID     *uint   `json:"userId,omitempty"`
}

type deleteResponse struct {
	Message string `json:"message"`
}

type alreadyOpenResponse struct {
	Error    string `json:"error"`
	TicketID uint   `json:"ticketId"`
}

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(gr chi.Router) {
		gr.Use(auth.IsAuth(h.AccessSecret))
		gr.Get("/tickets", httpx.Wrap(h.index))
		gr.Get("/tickets/{ticketId}", httpx.Wrap(h.show))
		gr.Post("/tickets", httpx.Wrap(h.store))
		gr.Put("/tickets/{ticketId}", httpx.Wrap(h.update))
		gr.With(auth.IsAdmin).Delete("/tickets/{ticketId}", httpx.Wrap(h.remove))
	})
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	params, appErr := parseListParams(r)
	if appErr != nil {
		return appErr
	}
	items, count, hasMore, err := h.Deps.List(r.Context(), params, &actor)
	if err != nil {
		return err
	}
	dtos := make([]TicketDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, Serialize(&items[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Tickets: dtos, Count: count, HasMore: hasMore})
	return nil
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) error {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	id, appErr := parseTicketID(r)
	if appErr != nil {
		return appErr
	}
	t, err := h.Deps.Show(r.Context(), id, &actor)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(t))
	return nil
}

func (h *Handler) store(w http.ResponseWriter, r *http.Request) error {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	var req storeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	if req.ContactID == 0 {
		return errors.New(errInvalidInput, http.StatusBadRequest)
	}
	if req.WhatsappID == nil {
		return errors.New(errNoDefaultWhatsapp, http.StatusBadRequest)
	}

	var existing Ticket
	err := h.Deps.DB.WithContext(r.Context()).
		Where("contact_id = ? AND whatsapp_id = ? AND status IN ?", req.ContactID, *req.WhatsappID, []string{statusPending, statusOpen}).
		First(&existing).Error
	if err == nil {
		httpx.WriteJSON(w, http.StatusConflict, alreadyOpenResponse{Error: errTicketAlreadyOpen, TicketID: existing.ID})
		return nil
	}
	if !stdErrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	var contactRow contact.Contact
	if cErr := h.Deps.DB.WithContext(r.Context()).First(&contactRow, req.ContactID).Error; cErr != nil {
		if stdErrors.Is(cErr, gorm.ErrRecordNotFound) {
			return errors.New(errInvalidInput, http.StatusBadRequest)
		}
		return errors.Wrap(cErr, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	status := statusOpen
	if req.Status != nil {
		if !validStatus(*req.Status) {
			return errors.New(errInvalidStatus, http.StatusBadRequest)
		}
		status = *req.Status
	}
	userID := actor.ID
	if req.UserID != nil {
		userID = *req.UserID
	}

	fresh := &Ticket{
		ContactID:      req.ContactID,
		WhatsappID:     *req.WhatsappID,
		Status:         status,
		UserID:         &userID,
		IsGroup:        contactRow.IsGroup,
		UnreadMessages: 0,
	}
	if cErr := h.Deps.DB.WithContext(r.Context()).Create(fresh).Error; cErr != nil {
		return errors.Wrap(cErr, "ERR_DB_INSERT", http.StatusInternalServerError)
	}

	reloaded, appErr := h.Deps.loadByID(r.Context(), fresh.ID)
	if appErr != nil {
		return appErr
	}
	dto := Serialize(reloaded)
	emitCreate(h.Deps.WS, dto)
	httpx.WriteJSON(w, http.StatusCreated, dto)
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	id, appErr := parseTicketID(r)
	if appErr != nil {
		return appErr
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	data := UpdateData{
		Status:  req.Status,
		UserID:  req.UserID,
		QueueID: req.QueueID,
	}
	t, err := h.Deps.Update(r.Context(), id, data, &actor)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(t))
	return nil
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) error {
	actor, ok := auth.UserFromContext(r.Context())
	if !ok {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}
	id, appErr := parseTicketID(r)
	if appErr != nil {
		return appErr
	}
	if err := h.Deps.Delete(r.Context(), id, &actor); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, deleteResponse{Message: "Ticket deleted"})
	return nil
}

func parseTicketID(r *http.Request) (uint, *errors.AppError) {
	raw := chi.URLParam(r, "ticketId")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	return uint(parsed), nil
}

func parseListParams(r *http.Request) (ListParams, *errors.AppError) {
	q := r.URL.Query()
	params := ListParams{
		SearchParam: q.Get("searchParam"),
	}
	if pn, err := strconv.Atoi(q.Get("pageNumber")); err == nil && pn > 0 {
		params.PageNumber = pn
	} else {
		params.PageNumber = 1
	}
	if statusRaw := q.Get("status"); statusRaw != "" {
		params.Status = splitCSV(statusRaw)
	}
	if queueRaw := q.Get("queueIds"); queueRaw != "" {
		ids, appErr := parseQueueIDs(queueRaw)
		if appErr != nil {
			return params, appErr
		}
		params.QueueIDs = ids
	}
	if userRaw := q.Get("userId"); userRaw != "" {
		uid, err := strconv.ParseUint(userRaw, 10, 64)
		if err != nil {
			return params, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
		}
		v := uint(uid)
		params.UserID = &v
	}
	if unreadRaw := q.Get("withUnreadMessages"); unreadRaw != "" {
		b := unreadRaw == "true"
		params.WithUnreadMessages = &b
	}
	if showAll := q.Get("showAll"); showAll == "true" {
		params.ShowAll = true
	}
	if dateRaw := q.Get("date"); dateRaw != "" {
		d, derr := parseDateParam(dateRaw)
		if derr != nil {
			return params, derr
		}
		params.Date = &d
	}
	return params, nil
}

func parseQueueIDs(raw string) ([]*uint, *errors.AppError) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		var jsonIDs []any
		if err := json.Unmarshal([]byte(trimmed), &jsonIDs); err != nil {
			return nil, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
		}
		out := make([]*uint, 0, len(jsonIDs))
		for _, item := range jsonIDs {
			if item == nil {
				out = append(out, nil)
				continue
			}
			switch v := item.(type) {
			case float64:
				if v < 0 {
					return nil, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
				}
				id := uint(v)
				out = append(out, &id)
			case string:
				if strings.EqualFold(v, "null") {
					out = append(out, nil)
					continue
				}
				parsed, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return nil, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
				}
				id := uint(parsed)
				out = append(out, &id)
			default:
				return nil, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
			}
		}
		return out, nil
	}
	parts := splitCSV(raw)
	out := make([]*uint, 0, len(parts))
	for _, p := range parts {
		if strings.EqualFold(p, "null") {
			out = append(out, nil)
			continue
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
		}
		id := uint(v)
		out = append(out, &id)
	}
	return out, nil
}

func parseDateParam(raw string) (time.Time, *errors.AppError) {
	layouts := []string{
		"2006-01-02",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if d, err := time.Parse(layout, raw); err == nil {
			return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}
