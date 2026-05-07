package queue

import (
	"context"
	stdErrors "errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const (
	wsChannelGlobal     = "global"
	wsEventCreated      = "queue.created"
	wsEventUpdated      = "queue.updated"
	wsEventDeleted      = "queue.deleted"
	uniqueViolationCode = "23505"
	nameUniqueIndex     = "queues_name_uniq"
	colorUniqueIndex    = "queues_color_uniq"
	errNameDuplicated   = "ERR_QUEUE_NAME_ALREADY_EXISTS"
	errColorDuplicated  = "ERR_QUEUE_COLOR_ALREADY_EXISTS"
	errQueueNotFound    = "ERR_QUEUE_NOT_FOUND"
	errInvalidQueue     = "ERR_INVALID_QUEUE"
)

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type Deps struct {
	DB *gorm.DB
	WS WSPublisher
}

func (d *Deps) Create(ctx context.Context, req CreateRequest) (*Queue, *errors.AppError) {
	trimCreate(&req)
	if err := validateCreate(&req); err != nil {
		return nil, err
	}
	entity := &Queue{
		Name:            req.Name,
		Color:           req.Color,
		GreetingMessage: req.GreetingMessage,
	}
	if err := d.DB.WithContext(ctx).Create(entity).Error; err != nil {
		if appErr := translateUniqueViolation(err); appErr != nil {
			return nil, appErr
		}
		return nil, errors.Wrap(err, "ERR_DB_INSERT", http.StatusInternalServerError)
	}
	d.publish(wsChannelGlobal, wsEventCreated, map[string]any{"queue": Serialize(entity)})
	return entity, nil
}

func (d *Deps) Show(ctx context.Context, id uint) (*Queue, *errors.AppError) {
	var entity Queue
	if err := d.DB.WithContext(ctx).First(&entity, id).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errQueueNotFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &entity, nil
}

func (d *Deps) List(ctx context.Context) ([]Queue, *errors.AppError) {
	var items []Queue
	if err := d.DB.WithContext(ctx).Order("name ASC").Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return items, nil
}

func (d *Deps) Update(ctx context.Context, id uint, req UpdateRequest) (*Queue, *errors.AppError) {
	trimUpdate(&req)
	if err := validateUpdate(&req); err != nil {
		return nil, err
	}
	existing, appErr := d.Show(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if req.Name != nil && *req.Name != existing.Name {
		taken, err := d.uniqueFieldTaken(ctx, "name", *req.Name, id)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, errors.New(errNameDuplicated, http.StatusBadRequest)
		}
	}
	if req.Color != nil && *req.Color != existing.Color {
		taken, err := d.uniqueFieldTaken(ctx, "color", *req.Color, id)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, errors.New(errColorDuplicated, http.StatusBadRequest)
		}
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.GreetingMessage != nil {
		updates["greeting_message"] = *req.GreetingMessage
	}
	if len(updates) > 0 {
		if err := d.DB.WithContext(ctx).Model(&Queue{}).
			Where("id = ?", id).
			Updates(updates).Error; err != nil {
			if appErr := translateUniqueViolation(err); appErr != nil {
				return nil, appErr
			}
			return nil, errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
		}
	}
	reloaded, appErr := d.Show(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	d.publish(wsChannelGlobal, wsEventUpdated, map[string]any{"queue": Serialize(reloaded)})
	return reloaded, nil
}

func (d *Deps) Delete(ctx context.Context, id uint) *errors.AppError {
	res := d.DB.WithContext(ctx).Delete(&Queue{}, id)
	if res.Error != nil {
		return errors.Wrap(res.Error, "ERR_DB_DELETE", http.StatusInternalServerError)
	}
	if res.RowsAffected == 0 {
		return errors.New(errQueueNotFound, http.StatusNotFound)
	}
	d.publish(wsChannelGlobal, wsEventDeleted, map[string]any{"queueId": id})
	return nil
}

func (d *Deps) uniqueFieldTaken(ctx context.Context, column, value string, id uint) (bool, *errors.AppError) {
	var count int64
	if err := d.DB.WithContext(ctx).Model(&Queue{}).
		Where(column+" = ? AND id != ?", value, id).
		Count(&count).Error; err != nil {
		return false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return count > 0, nil
}

func (d *Deps) publish(channel, event string, data any) {
	if d.WS == nil {
		return
	}
	d.WS.Publish(channel, event, data)
}

func AssociateWhatsappQueues(ctx context.Context, db *gorm.DB, whatsappID uint, queueIDs []uint) *errors.AppError {
	return associateQueues(ctx, db, "whatsapp_queues", "whatsapp_id", whatsappID, queueIDs, func(queueID uint) any {
		return WhatsappQueue{WhatsappID: whatsappID, QueueID: queueID}
	})
}

func AssociateUserQueues(ctx context.Context, db *gorm.DB, userID uint, queueIDs []uint) *errors.AppError {
	return associateQueues(ctx, db, "user_queues", "user_id", userID, queueIDs, func(queueID uint) any {
		return UserQueue{UserID: userID, QueueID: queueID}
	})
}

func GetQueueIDsByUser(ctx context.Context, db *gorm.DB, userID uint) ([]uint, *errors.AppError) {
	ids := []uint{}
	if err := db.WithContext(ctx).Model(&UserQueue{}).
		Where("user_id = ?", userID).
		Pluck("queue_id", &ids).Error; err != nil {
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	if ids == nil {
		ids = []uint{}
	}
	return ids, nil
}

func associateQueues(ctx context.Context, db *gorm.DB, table, column string, ownerID uint, queueIDs []uint, build func(uint) any) *errors.AppError {
	deduped := dedupeIDs(queueIDs)
	return wrapAppError(db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(deduped) > 0 {
			var count int64
			if err := tx.Model(&Queue{}).Where("id IN ?", deduped).Count(&count).Error; err != nil {
				return errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
			}
			if int(count) != len(deduped) {
				return errors.New(errInvalidQueue, http.StatusBadRequest)
			}
		}
		if err := tx.Exec("DELETE FROM "+table+" WHERE "+column+" = ?", ownerID).Error; err != nil {
			return errors.Wrap(err, "ERR_DB_DELETE", http.StatusInternalServerError)
		}
		if len(deduped) == 0 {
			return nil
		}
		rows := make([]any, 0, len(deduped))
		for _, qID := range deduped {
			rows = append(rows, build(qID))
		}
		for _, row := range rows {
			if err := tx.Create(row).Error; err != nil {
				return errors.Wrap(err, "ERR_DB_INSERT", http.StatusInternalServerError)
			}
		}
		return nil
	}))
}

func dedupeIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func wrapAppError(err error) *errors.AppError {
	if err == nil {
		return nil
	}
	var appErr *errors.AppError
	if stdErrors.As(err, &appErr) {
		return appErr
	}
	return errors.Wrap(err, "ERR_DB_TRANSACTION", http.StatusInternalServerError)
}

func translateUniqueViolation(err error) *errors.AppError {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return nil
	}
	if pgErr.Code != uniqueViolationCode {
		return nil
	}
	switch pgErr.ConstraintName {
	case nameUniqueIndex:
		return errors.New(errNameDuplicated, http.StatusBadRequest)
	case colorUniqueIndex:
		return errors.New(errColorDuplicated, http.StatusBadRequest)
	}
	return nil
}
