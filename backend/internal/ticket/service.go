package ticket

import (
	"context"
	stdErrors "errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const (
	statusPending = "pending"
	statusOpen    = "open"
	statusClosed  = "closed"

	errTicketNotFound    = "ERR_TICKET_NOT_FOUND"
	errNoPermission      = "ERR_NO_PERMISSION"
	errInvalidStatus     = "ERR_INVALID_STATUS"
	errTicketAlreadyOpen = "ERR_TICKET_ALREADY_OPEN"
	errNoDefaultWhatsapp = "ERR_NO_DEFAULT_WHATSAPP"
	errInvalidInput      = "ERR_INVALID_INPUT"
)

type UserService interface {
	GetQueueIDs(ctx context.Context, userID uint) ([]uint, error)
}

type Deps struct {
	DB          *gorm.DB
	WS          WSPublisher
	UserService UserService
}

func (d *Deps) Show(ctx context.Context, id uint, actor *auth.UserClaims) (*Ticket, *errors.AppError) {
	t, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	queues, err := d.userQueueIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !canSee(actor, t, queues) {
		return nil, errors.New(errNoPermission, http.StatusForbidden)
	}
	return t, nil
}

func (d *Deps) Update(ctx context.Context, id uint, data UpdateData, actor *auth.UserClaims) (*Ticket, *errors.AppError) {
	if data.Status != nil && !validStatus(*data.Status) {
		return nil, errors.New(errInvalidStatus, http.StatusBadRequest)
	}
	t, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	queues, qErr := d.userQueueIDs(ctx, actor)
	if qErr != nil {
		return nil, qErr
	}
	if !canModify(actor, t, queues) {
		return nil, errors.New(errNoPermission, http.StatusForbidden)
	}

	oldStatus := t.Status
	updates := map[string]any{}

	if data.LastMessage != nil {
		updates["last_message"] = *data.LastMessage
	}
	if data.UnreadMessages != nil {
		updates["unread_messages"] = *data.UnreadMessages
	}
	if data.QueueID != nil {
		if *data.QueueID == nil {
			updates["queue_id"] = gorm.Expr("NULL")
		} else {
			updates["queue_id"] = **data.QueueID
		}
	}

	statusChange := data.Status != nil && *data.Status != t.Status
	autoRead := false

	switch {
	case data.Status != nil && *data.Status == statusOpen:
		if data.UserID != nil {
			if *data.UserID == nil {
				if actor.Profile != "admin" {
					return nil, errors.New(errNoPermission, http.StatusForbidden)
				}
				updates["user_id"] = gorm.Expr("NULL")
			} else {
				assignedID := **data.UserID
				if assignedID != actor.ID && actor.Profile != "admin" {
					return nil, errors.New(errNoPermission, http.StatusForbidden)
				}
				updates["user_id"] = assignedID
			}
		} else if t.UserID == nil || *t.UserID != actor.ID {
			updates["user_id"] = actor.ID
		}
		if statusChange {
			updates["status"] = statusOpen
		}
		if statusChange && t.UnreadMessages > 0 {
			autoRead = true
		}

	case data.Status != nil && *data.Status == statusPending:
		if statusChange {
			updates["status"] = statusPending
			updates["user_id"] = gorm.Expr("NULL")
		}

	case data.Status != nil:
		if statusChange {
			updates["status"] = *data.Status
		}
		if data.UserID != nil {
			applyUserIDUpdate(updates, data.UserID, actor)
			if rejection := assignRejection(data.UserID, actor); rejection != nil {
				return nil, rejection
			}
		}

	default:
		if data.UserID != nil {
			applyUserIDUpdate(updates, data.UserID, actor)
			if rejection := assignRejection(data.UserID, actor); rejection != nil {
				return nil, rejection
			}
		}
	}

	if len(updates) == 0 && !autoRead {
		return t, nil
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if autoRead {
			if appErr := SetMessagesAsRead(tx, id); appErr != nil {
				return appErr
			}
		}
		return nil
	})
	if txErr != nil {
		var appErr *errors.AppError
		if stdErrors.As(txErr, &appErr) {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}

	reloaded, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	emitUpdate(d.WS, Serialize(reloaded), oldStatus)
	return reloaded, nil
}

func (d *Deps) Delete(ctx context.Context, id uint, actor *auth.UserClaims) *errors.AppError {
	if actor == nil || actor.Profile != "admin" {
		return errors.New(errNoPermission, http.StatusForbidden)
	}
	t, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return appErr
	}
	dto := Serialize(t)
	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&Ticket{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if txErr != nil {
		if stdErrors.Is(txErr, gorm.ErrRecordNotFound) {
			return errors.New(errTicketNotFound, http.StatusNotFound)
		}
		return errors.Wrap(txErr, "ERR_DB_DELETE", http.StatusInternalServerError)
	}
	emitDelete(d.WS, dto)
	return nil
}

func (d *Deps) UpdateLastMessage(ctx context.Context, ticketID uint, body string) *errors.AppError {
	t, appErr := d.loadByID(ctx, ticketID)
	if appErr != nil {
		return appErr
	}
	oldStatus := t.Status
	if err := d.DB.WithContext(ctx).Model(&Ticket{}).
		Where("id = ?", ticketID).
		Update("last_message", body).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	reloaded, appErr := d.loadByID(ctx, ticketID)
	if appErr != nil {
		return appErr
	}
	emitUpdate(d.WS, Serialize(reloaded), oldStatus)
	return nil
}

func (d *Deps) CountByContactAndStatus(ctx context.Context, contactID uint, statuses []string) (int64, *errors.AppError) {
	q := d.DB.WithContext(ctx).Model(&Ticket{}).Where("contact_id = ?", contactID)
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return count, nil
}

func (d *Deps) LoadByID(ctx context.Context, id uint) (*Ticket, *errors.AppError) {
	return d.loadByID(ctx, id)
}

func (d *Deps) loadByID(ctx context.Context, id uint) (*Ticket, *errors.AppError) {
	var t Ticket
	err := d.DB.WithContext(ctx).
		Preload("Contact").
		Preload("Contact.ExtraInfo").
		Preload("User").
		Preload("Queue").
		Preload("Whatsapp").
		First(&t, id).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errTicketNotFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &t, nil
}

func (d *Deps) userQueueIDs(ctx context.Context, actor *auth.UserClaims) ([]uint, *errors.AppError) {
	if actor == nil {
		return nil, errors.New(errNoPermission, http.StatusForbidden)
	}
	if d.UserService == nil {
		return []uint{}, nil
	}
	ids, err := d.UserService.GetQueueIDs(ctx, actor.ID)
	if err != nil {
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	if ids == nil {
		ids = []uint{}
	}
	return ids, nil
}

func validStatus(s string) bool {
	return s == statusPending || s == statusOpen || s == statusClosed
}

func applyUserIDUpdate(updates map[string]any, userID **uint, actor *auth.UserClaims) {
	if userID == nil {
		return
	}
	if *userID == nil {
		if actor.Profile == "admin" {
			updates["user_id"] = gorm.Expr("NULL")
		}
		return
	}
	updates["user_id"] = **userID
}

func assignRejection(userID **uint, actor *auth.UserClaims) *errors.AppError {
	if userID == nil {
		return nil
	}
	if *userID == nil {
		if actor.Profile != "admin" {
			return errors.New(errNoPermission, http.StatusForbidden)
		}
		return nil
	}
	if **userID != actor.ID && actor.Profile != "admin" {
		return errors.New(errNoPermission, http.StatusForbidden)
	}
	return nil
}
