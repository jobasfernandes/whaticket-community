package user

import (
	"context"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
)

type QueueLookup struct {
	DB *gorm.DB
}

func NewQueueLookup(db *gorm.DB) *QueueLookup {
	return &QueueLookup{DB: db}
}

func (q *QueueLookup) GetQueueIDs(ctx context.Context, userID uint) ([]uint, error) {
	ids, appErr := queue.GetQueueIDsByUser(ctx, q.DB, userID)
	if appErr != nil {
		return nil, appErr
	}
	return ids, nil
}
