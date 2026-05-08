package ticket

import (
	"context"
	stdErrors "errors"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/contact"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const reopenWindow = 2 * time.Hour

const (
	actionCreated  = "created"
	actionReopened = "reopened"
	actionBumped   = "bumped"
)

func (d *Deps) FindOrCreate(ctx context.Context, contactRef *contact.Contact, whatsappID uint, unreadMessages int, groupContact *contact.Contact) (*Ticket, *errors.AppError) {
	if contactRef == nil && groupContact == nil {
		return nil, errors.New(errInvalidInput, http.StatusBadRequest)
	}
	if whatsappID == 0 {
		return nil, errors.New(errInvalidInput, http.StatusBadRequest)
	}

	isGroup := groupContact != nil
	var lookupContactID uint
	if isGroup {
		lookupContactID = groupContact.ID
	} else {
		lookupContactID = contactRef.ID
	}

	var resultID uint
	var action string
	var oldStatus string

	lockErr := withContactLock(ctx, d.DB, lookupContactID, whatsappID, func(tx *gorm.DB) error {
		var existing Ticket
		err := tx.Where("contact_id = ? AND whatsapp_id = ? AND status IN ?", lookupContactID, whatsappID, []string{statusOpen, statusPending}).
			Order("updated_at DESC").
			First(&existing).Error
		if err == nil {
			oldStatus = existing.Status
			if uErr := tx.Model(&Ticket{}).Where("id = ?", existing.ID).
				Update("unread_messages", unreadMessages).Error; uErr != nil {
				return uErr
			}
			resultID = existing.ID
			action = actionBumped
			return nil
		}
		if !stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var reopenable Ticket
		var rErr error
		if isGroup {
			rErr = tx.Where("contact_id = ? AND whatsapp_id = ?", lookupContactID, whatsappID).
				Order("updated_at DESC").
				First(&reopenable).Error
		} else {
			cutoff := time.Now().Add(-reopenWindow)
			rErr = tx.Where("contact_id = ? AND whatsapp_id = ? AND updated_at >= ?", lookupContactID, whatsappID, cutoff).
				Order("updated_at DESC").
				First(&reopenable).Error
		}
		if rErr == nil {
			oldStatus = reopenable.Status
			if uErr := tx.Model(&Ticket{}).Where("id = ?", reopenable.ID).
				Updates(map[string]any{
					"status":          statusPending,
					"user_id":         gorm.Expr("NULL"),
					"unread_messages": unreadMessages,
				}).Error; uErr != nil {
				return uErr
			}
			resultID = reopenable.ID
			action = actionReopened
			return nil
		}
		if !stdErrors.Is(rErr, gorm.ErrRecordNotFound) {
			return rErr
		}

		fresh := &Ticket{
			ContactID:      lookupContactID,
			WhatsappID:     whatsappID,
			Status:         statusPending,
			IsGroup:        isGroup,
			UnreadMessages: unreadMessages,
		}
		if cErr := tx.Create(fresh).Error; cErr != nil {
			return cErr
		}
		resultID = fresh.ID
		action = actionCreated
		return nil
	})
	if lockErr != nil {
		var appErr *errors.AppError
		if stdErrors.As(lockErr, &appErr) {
			return nil, appErr
		}
		return nil, errors.Wrap(lockErr, "ERR_DB_TRANSACTION", http.StatusInternalServerError)
	}

	reloaded, appErr := d.loadByID(ctx, resultID)
	if appErr != nil {
		return nil, appErr
	}
	dto := Serialize(reloaded)
	switch action {
	case actionCreated:
		emitCreate(d.WS, dto)
	case actionReopened:
		emitUpdate(d.WS, dto, oldStatus)
	case actionBumped:
		emitUpdate(d.WS, dto, dto.Status)
	}
	return reloaded, nil
}
