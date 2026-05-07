package message

import (
	"context"
	stdErrors "errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const (
	wsEventAppMessageCreate = "appMessage.create"
	wsEventAppMessageDelete = "appMessage.delete"
	wsChannelTicketPrefix   = "ticket:"
	wsChannelNotification   = "notification"
	wsActionCreate          = "create"
	wsActionDelete          = "delete"
	errInvalidInput         = "ERR_INVALID_INPUT"
	errMessageNotFound      = "ERR_MESSAGE_NOT_FOUND"
	errNoPermission         = "ERR_NO_PERMISSION"
	profileAdmin            = "admin"
)

type Deps struct {
	DB            *gorm.DB
	WS            WSPublisher
	TicketSvc     TicketService
	Sender        Sender
	MediaUploader MediaUploader
}

type MediaUploader interface {
	Upload(ctx context.Context, objectKey string, data []byte, mimeType string) (string, error)
}

type Sender interface {
	SendText(ctx context.Context, whatsappID uint, to, body string, quotedID string) (string, time.Time, *errors.AppError)
	SendImage(ctx context.Context, whatsappID uint, to, dataURL, caption, mimeType, quotedID string) (string, time.Time, *errors.AppError)
	SendAudio(ctx context.Context, whatsappID uint, to, dataURL, mimeType string, ptt bool, quotedID string) (string, time.Time, *errors.AppError)
	SendVideo(ctx context.Context, whatsappID uint, to, dataURL, caption, mimeType, quotedID string) (string, time.Time, *errors.AppError)
	SendDocument(ctx context.Context, whatsappID uint, to, dataURL, fileName, caption, mimeType, quotedID string) (string, time.Time, *errors.AppError)
	SendSticker(ctx context.Context, whatsappID uint, to, dataURL, quotedID string) (string, time.Time, *errors.AppError)
}

func (d *Deps) Create(ctx context.Context, data MessageData) (*Message, *errors.AppError) {
	if data.ID == "" || data.TicketID == 0 {
		return nil, errors.New(errInvalidInput, http.StatusBadRequest)
	}
	mediaType := data.MediaType
	if mediaType == "" {
		mediaType = "chat"
	}
	msg := Message{
		ID:          data.ID,
		TicketID:    data.TicketID,
		ContactID:   data.ContactID,
		Body:        data.Body,
		MediaType:   mediaType,
		MediaURL:    data.MediaURL,
		FromMe:      data.FromMe,
		Read:        data.Read,
		Ack:         data.Ack,
		QuotedMsgID: data.QuotedMsgID,
	}
	err := d.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"body":       msg.Body,
			"media_url":  msg.MediaURL,
			"media_type": msg.MediaType,
			"ack":        gorm.Expr("GREATEST(messages.ack, EXCLUDED.ack)"),
			"read":       msg.Read,
			"updated_at": time.Now(),
		}),
	}).Create(&msg).Error
	if err != nil {
		return nil, errors.Wrap(err, "ERR_DB_INSERT", http.StatusInternalServerError)
	}
	reloaded, appErr := d.findByID(ctx, msg.ID)
	if appErr != nil {
		return nil, appErr
	}
	if d.TicketSvc != nil {
		if upErr := d.TicketSvc.UpdateLastMessage(ctx, data.TicketID, reloaded.Body); upErr != nil {
			return nil, upErr
		}
	}
	payload := map[string]any{
		"action":  wsActionCreate,
		"message": Serialize(reloaded),
	}
	if d.TicketSvc != nil {
		if t, terr := d.TicketSvc.LoadByID(ctx, data.TicketID); terr == nil && t != nil {
			payload["ticket"] = d.TicketSvc.SerializeTicket(t)
		}
	}
	d.publish(wsChannelTicketPrefix+strconv.FormatUint(uint64(data.TicketID), 10), wsEventAppMessageCreate, payload)
	d.publish(wsChannelNotification, wsEventAppMessageCreate, payload)
	return reloaded, nil
}

func (d *Deps) Delete(ctx context.Context, messageID string, actor *auth.UserClaims) *errors.AppError {
	if messageID == "" {
		return errors.New(errInvalidInput, http.StatusBadRequest)
	}
	if actor == nil {
		return errors.New(errNoPermission, http.StatusForbidden)
	}
	msg, appErr := d.findByID(ctx, messageID)
	if appErr != nil {
		return appErr
	}
	ticket, appErr := d.TicketSvc.Show(ctx, msg.TicketID, actor)
	if appErr != nil {
		return appErr
	}
	if !canDelete(actor, ticket) {
		return errors.New(errNoPermission, http.StatusForbidden)
	}
	previousBody := msg.Body
	ticketID := msg.TicketID
	var newBody string
	var lastMessageChanged bool
	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if delErr := tx.Where("id = ?", messageID).Delete(&Message{}).Error; delErr != nil {
			return errors.Wrap(delErr, "ERR_DB_DELETE", http.StatusInternalServerError)
		}
		var newest Message
		findErr := tx.Where("ticket_id = ?", ticketID).
			Order("created_at DESC, id DESC").
			Limit(1).
			Take(&newest).Error
		switch {
		case findErr == nil:
			newBody = newest.Body
		case stdErrors.Is(findErr, gorm.ErrRecordNotFound):
			newBody = ""
		default:
			return errors.Wrap(findErr, "ERR_DB_QUERY", http.StatusInternalServerError)
		}
		lastMessageChanged = previousBody != newBody
		return nil
	})
	if txErr != nil {
		var appErr *errors.AppError
		if stdErrors.As(txErr, &appErr) {
			return appErr
		}
		return errors.Wrap(txErr, "ERR_DB_TRANSACTION", http.StatusInternalServerError)
	}
	if lastMessageChanged && d.TicketSvc != nil {
		if upErr := d.TicketSvc.UpdateLastMessage(ctx, ticketID, newBody); upErr != nil {
			return upErr
		}
	}
	deletePayload := map[string]any{
		"action":    wsActionDelete,
		"messageId": messageID,
	}
	if d.TicketSvc != nil {
		if t, terr := d.TicketSvc.LoadByID(ctx, ticketID); terr == nil && t != nil {
			deletePayload["ticket"] = d.TicketSvc.SerializeTicket(t)
		}
	}
	d.publish(
		wsChannelTicketPrefix+strconv.FormatUint(uint64(ticketID), 10),
		wsEventAppMessageDelete,
		deletePayload,
	)
	slog.Default().Info("message deleted",
		"actor_id", actor.ID,
		"message_id", messageID,
		"ticket_id", ticketID,
		"was_last_message", lastMessageChanged,
	)
	return nil
}

func (d *Deps) FindByID(ctx context.Context, id string) (*Message, *errors.AppError) {
	return d.findByID(ctx, id)
}

func (d *Deps) findByID(ctx context.Context, id string) (*Message, *errors.AppError) {
	var msg Message
	err := d.DB.WithContext(ctx).
		Preload("Contact").
		Preload("QuotedMsg.Contact").
		Where("id = ?", id).
		Take(&msg).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errMessageNotFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &msg, nil
}

func (d *Deps) publish(channel, event string, data any) {
	if d.WS == nil {
		return
	}
	d.WS.Publish(channel, event, data)
}

func canDelete(actor *auth.UserClaims, ticket TicketLike) bool {
	if actor.Profile == profileAdmin {
		return true
	}
	owner := ticket.GetUserID()
	if owner == nil {
		return false
	}
	return *owner == actor.ID
}
