package ticket

import (
	stdErrors "errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
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

func isMissingTableError(err error) bool {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgUndefinedTable
}
