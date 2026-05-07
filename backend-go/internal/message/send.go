package message

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/contact"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

const (
	errMediaUnsupported = "ERR_MEDIA_NOT_SUPPORTED"
	errNoSender         = "ERR_SENDER_NOT_CONFIGURED"
)

type sendBody struct {
	Body      string         `json:"body"`
	FromMe    bool           `json:"fromMe"`
	Read      any            `json:"read"`
	MediaURL  string         `json:"mediaUrl"`
	QuotedMsg map[string]any `json:"quotedMsg"`
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) error {
	ticketID, appErr := parseTicketID(r)
	if appErr != nil {
		return appErr
	}
	actor := claimsPointer(r)
	if actor == nil {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return errors.New(errMediaUnsupported, http.StatusNotImplemented)
	}

	var req sendBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return errors.New(errInvalidInput, http.StatusBadRequest)
	}

	ticketLike, appErr := h.Deps.TicketSvc.Show(r.Context(), ticketID, actor)
	if appErr != nil {
		return appErr
	}

	contactNumber, appErr := h.Deps.lookupContactNumber(r.Context(), ticketLike.GetContactID())
	if appErr != nil {
		return appErr
	}

	if h.Deps.Sender == nil {
		return errors.New(errNoSender, http.StatusServiceUnavailable)
	}

	quotedID := extractQuotedID(req.QuotedMsg)
	id, _, sendErr := h.Deps.Sender.SendText(r.Context(), ticketLike.GetWhatsappID(), contactNumber, body, quotedID)
	if sendErr != nil {
		return sendErr
	}

	contactID := ticketLike.GetContactID()
	data := MessageData{
		ID:        id,
		TicketID:  ticketID,
		ContactID: &contactID,
		Body:      body,
		MediaType: "chat",
		MediaURL:  req.MediaURL,
		FromMe:    true,
		Read:      true,
		Ack:       0,
	}
	if quotedID != "" {
		q := quotedID
		data.QuotedMsgID = &q
	}

	created, appErr := h.Deps.Create(r.Context(), data)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusCreated, Serialize(created))
	return nil
}

func (d *Deps) lookupContactNumber(ctx context.Context, contactID uint) (string, *errors.AppError) {
	if contactID == 0 {
		return "", errors.New(errInvalidInput, http.StatusBadRequest)
	}
	var c contact.Contact
	err := d.DB.WithContext(ctx).Select("number", "is_group").Where("id = ?", contactID).Take(&c).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("ERR_NO_CONTACT_FOUND", http.StatusNotFound)
		}
		return "", errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	number := c.Number
	if number == "" {
		return "", errors.New("ERR_NO_CONTACT_NUMBER", http.StatusBadRequest)
	}
	return number, nil
}

func extractQuotedID(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["id"].(string); ok {
		return v
	}
	return ""
}
