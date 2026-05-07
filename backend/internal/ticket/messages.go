package ticket

import (
	"context"
	stdErrors "errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const pgUndefinedTable = "42P01"

func SetMessagesAsRead(tx *gorm.DB, ticketID uint) *errors.AppError {
	if err := tx.Exec("UPDATE messages SET read = true WHERE ticket_id = ?", ticketID).Error; err != nil {
		if !isMissingTableError(err) {
			return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
		}
	}
	if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).Update("unread_messages", 0).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	return nil
}

func (d *Deps) SetMessagesAsRead(ctx context.Context, ticketID uint) *errors.AppError {
	t, appErr := d.loadByID(ctx, ticketID)
	if appErr != nil {
		return appErr
	}
	oldStatus := t.Status
	if appErr := SetMessagesAsRead(d.DB.WithContext(ctx), ticketID); appErr != nil {
		return appErr
	}
	reloaded, appErr := d.loadByID(ctx, ticketID)
	if appErr != nil {
		return appErr
	}
	emitUpdate(d.WS, Serialize(reloaded), oldStatus)
	return nil
}

func (d *Deps) UpdateQueue(ctx context.Context, ticketID, queueID uint) *errors.AppError {
	t, appErr := d.loadByID(ctx, ticketID)
	if appErr != nil {
		return appErr
	}
	oldStatus := t.Status
	if err := d.DB.WithContext(ctx).Model(&Ticket{}).
		Where("id = ?", ticketID).
		Update("queue_id", queueID).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	reloaded, appErr := d.loadByID(ctx, ticketID)
	if appErr != nil {
		return appErr
	}
	emitUpdate(d.WS, Serialize(reloaded), oldStatus)
	return nil
}

func isMissingTableError(err error) bool {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgUndefinedTable
}
