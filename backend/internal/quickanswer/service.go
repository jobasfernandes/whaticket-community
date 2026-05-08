package quickanswer

import (
	"context"
	stdErrors "errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const (
	pageSize                 = 20
	wsChannelGlobal          = "global"
	wsEventCreated           = "quickAnswer.created"
	wsEventUpdated           = "quickAnswer.updated"
	wsEventDeleted           = "quickAnswer.deleted"
	wsActionCreate           = "create"
	wsActionUpdate           = "update"
	wsActionDelete           = "delete"
	uniqueViolationCode      = "23505"
	shortcutUniqueConstraint = "quick_answers_shortcut_uniq"
	errShortcutDuplicated    = "ERR__SHORTCUT_DUPLICATED"
	errNoQuickAnswersFound   = "ERR_NO_QUICK_ANSWERS_FOUND"
)

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type Deps struct {
	DB *gorm.DB
	WS WSPublisher
}

func (d *Deps) Create(ctx context.Context, req CreateRequest) (*QuickAnswer, *errors.AppError) {
	trim(&req)
	if err := validateCreate(&req); err != nil {
		return nil, err
	}
	entity := &QuickAnswer{Shortcut: req.Shortcut, Message: req.Message}
	if err := d.DB.WithContext(ctx).Create(entity).Error; err != nil {
		if isShortcutUniqueViolation(err) {
			return nil, errors.New(errShortcutDuplicated, http.StatusBadRequest)
		}
		return nil, errors.Wrap(err, "ERR_DB_INSERT", http.StatusInternalServerError)
	}
	d.publish(wsChannelGlobal, wsEventCreated, map[string]any{
		"action":      wsActionCreate,
		"quickAnswer": Serialize(entity),
	})
	return entity, nil
}

func (d *Deps) Show(ctx context.Context, id uint) (*QuickAnswer, *errors.AppError) {
	var entity QuickAnswer
	if err := d.DB.WithContext(ctx).First(&entity, id).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errNoQuickAnswersFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &entity, nil
}

func (d *Deps) List(ctx context.Context, searchParam string, pageNumber int) ([]QuickAnswer, int64, bool, *errors.AppError) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	query := d.DB.WithContext(ctx).Model(&QuickAnswer{})
	searchParam = strings.TrimSpace(searchParam)
	if searchParam != "" {
		pattern := "%" + escapeLike(strings.ToLower(searchParam)) + "%"
		query = query.Where("LOWER(message) ILIKE ?", pattern)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	var items []QuickAnswer
	offset := (pageNumber - 1) * pageSize
	if err := query.Order("message ASC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	hasMore := count > int64(pageNumber*pageSize)
	return items, count, hasMore, nil
}

func (d *Deps) Update(ctx context.Context, id uint, req UpdateRequest) (*QuickAnswer, *errors.AppError) {
	trimUpdate(&req)
	if err := validateUpdate(&req); err != nil {
		return nil, err
	}
	existing, appErr := d.Show(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if req.Shortcut != nil && *req.Shortcut != existing.Shortcut {
		var count int64
		if err := d.DB.WithContext(ctx).Model(&QuickAnswer{}).
			Where("shortcut = ? AND id != ?", *req.Shortcut, id).
			Count(&count).Error; err != nil {
			return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
		}
		if count > 0 {
			return nil, errors.New(errShortcutDuplicated, http.StatusBadRequest)
		}
	}
	updates := map[string]any{}
	if req.Shortcut != nil {
		updates["shortcut"] = *req.Shortcut
	}
	if req.Message != nil {
		updates["message"] = *req.Message
	}
	if len(updates) > 0 {
		if err := d.DB.WithContext(ctx).Model(&QuickAnswer{}).
			Where("id = ?", id).
			Updates(updates).Error; err != nil {
			if isShortcutUniqueViolation(err) {
				return nil, errors.New(errShortcutDuplicated, http.StatusBadRequest)
			}
			return nil, errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
		}
	}
	reloaded, appErr := d.Show(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	d.publish(wsChannelGlobal, wsEventUpdated, map[string]any{
		"action":      wsActionUpdate,
		"quickAnswer": Serialize(reloaded),
	})
	return reloaded, nil
}

func (d *Deps) Delete(ctx context.Context, id uint) *errors.AppError {
	res := d.DB.WithContext(ctx).Delete(&QuickAnswer{}, id)
	if res.Error != nil {
		return errors.Wrap(res.Error, "ERR_DB_DELETE", http.StatusInternalServerError)
	}
	if res.RowsAffected == 0 {
		return errors.New(errNoQuickAnswersFound, http.StatusNotFound)
	}
	d.publish(wsChannelGlobal, wsEventDeleted, map[string]any{
		"action":        wsActionDelete,
		"quickAnswerId": id,
	})
	return nil
}

func (d *Deps) publish(channel, event string, data any) {
	if d.WS == nil {
		return
	}
	d.WS.Publish(channel, event, data)
}

func isShortcutUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == shortcutUniqueConstraint
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
