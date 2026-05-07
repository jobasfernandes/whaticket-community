package message

import (
	"context"
	"net/http"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const listPageSize = 20

func (d *Deps) List(ctx context.Context, ticketID uint, pageNumber int) ([]Message, int64, bool, *errors.AppError) {
	if ticketID == 0 {
		return nil, 0, false, errors.New(errInvalidInput, http.StatusBadRequest)
	}
	if pageNumber < 1 {
		pageNumber = 1
	}
	var count int64
	if err := d.DB.WithContext(ctx).
		Model(&Message{}).
		Where("ticket_id = ?", ticketID).
		Count(&count).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	var messages []Message
	if err := d.DB.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC, id DESC").
		Preload("Contact").
		Preload("QuotedMsg.Contact").
		Offset((pageNumber - 1) * listPageSize).
		Limit(listPageSize).
		Find(&messages).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	hasMore := count > int64(pageNumber*listPageSize)
	return messages, count, hasMore, nil
}
