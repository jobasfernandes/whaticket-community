package contact

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
	pageSize             = 20
	wsChannelGlobal      = "global"
	wsEventCreated       = "contact.created"
	wsEventUpdated       = "contact.updated"
	wsEventDeleted       = "contact.deleted"
	wsActionCreate       = "create"
	wsActionUpdate       = "update"
	wsActionDelete       = "delete"
	pgUniqueViolation    = "23505"
	contactNumberUnique  = "contacts_number_uniq"
	errNoContactFound    = "ERR_NO_CONTACT_FOUND"
	errDuplicatedContact = "ERR_DUPLICATED_CONTACT"
)

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type Deps struct {
	DB *gorm.DB
	WS WSPublisher
}

func (d *Deps) CreateOrUpdate(ctx context.Context, req CreateOrUpdateRequest) (*Contact, *errors.AppError) {
	req.Number = NormalizeNumber(req.Number, req.IsGroup)
	req.Name = strings.TrimSpace(req.Name)

	if req.Number == "" && req.LID == "" {
		return nil, errors.New(errInvalidNumber, http.StatusBadRequest)
	}

	var surviving *Contact
	var action string

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		byNumber, err := findOneByField(tx, "number", req.Number)
		if err != nil {
			return err
		}
		byLID, err := findOneByField(tx, "lid", req.LID)
		if err != nil {
			return err
		}
		merged, mergeAction, mErr := merge(tx, byNumber, byLID, req)
		if mErr != nil {
			return mErr
		}
		surviving = merged
		action = mergeAction
		return nil
	})
	if txErr != nil {
		var appErr *errors.AppError
		if stdErrors.As(txErr, &appErr) {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_TRANSACTION", http.StatusInternalServerError)
	}

	reloaded, appErr := d.loadByID(ctx, surviving.ID)
	if appErr != nil {
		return nil, appErr
	}

	event := wsEventUpdated
	emitAction := wsActionUpdate
	if action == actionCreated {
		event = wsEventCreated
		emitAction = wsActionCreate
	}
	d.publish(wsChannelGlobal, event, map[string]any{
		"action":  emitAction,
		"contact": Serialize(reloaded),
	})
	return reloaded, nil
}

func (d *Deps) Create(ctx context.Context, req CreateRequest) (*Contact, *errors.AppError) {
	trimCreate(&req)
	if appErr := validateCreate(&req); appErr != nil {
		return nil, appErr
	}
	number := NormalizeNumber(req.Number, false)

	entity := &Contact{
		Name:    req.Name,
		Number:  number,
		Email:   req.Email,
		IsGroup: false,
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		for i := range req.ExtraInfo {
			row := &ContactCustomField{
				ContactID: entity.ID,
				Name:      req.ExtraInfo[i].Name,
				Value:     req.ExtraInfo[i].Value,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		if appErr := translateUniqueViolation(txErr); appErr != nil {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_INSERT", http.StatusInternalServerError)
	}

	reloaded, appErr := d.loadByID(ctx, entity.ID)
	if appErr != nil {
		return nil, appErr
	}
	d.publish(wsChannelGlobal, wsEventCreated, map[string]any{
		"action":  wsActionCreate,
		"contact": Serialize(reloaded),
	})
	return reloaded, nil
}

func (d *Deps) Show(ctx context.Context, id uint) (*Contact, *errors.AppError) {
	return d.loadByID(ctx, id)
}

func (d *Deps) Update(ctx context.Context, id uint, req UpdateRequest) (*Contact, *errors.AppError) {
	trimUpdate(&req)
	if appErr := validateUpdate(&req); appErr != nil {
		return nil, appErr
	}

	existing, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Number != nil {
		newNumber := NormalizeNumber(*req.Number, existing.IsGroup)
		if newNumber != existing.Number {
			taken, err := d.numberTakenByOther(ctx, newNumber, id)
			if err != nil {
				return nil, err
			}
			if taken {
				return nil, errors.New(errDuplicatedContact, http.StatusBadRequest)
			}
			updates["number"] = newNumber
		}
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&Contact{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if req.ExtraInfo != nil {
			if err := upsertExtraInfo(tx, id, *req.ExtraInfo); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		var appErr *errors.AppError
		if stdErrors.As(txErr, &appErr) {
			return nil, appErr
		}
		if appErr := translateUniqueViolation(txErr); appErr != nil {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}

	reloaded, loadErr := d.loadByID(ctx, id)
	if loadErr != nil {
		return nil, loadErr
	}
	d.publish(wsChannelGlobal, wsEventUpdated, map[string]any{
		"action":  wsActionUpdate,
		"contact": Serialize(reloaded),
	})
	return reloaded, nil
}

func (d *Deps) Delete(ctx context.Context, id uint) *errors.AppError {
	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("contact_id = ?", id).Delete(&ContactCustomField{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&Contact{}, id)
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
			return errors.New(errNoContactFound, http.StatusNotFound)
		}
		return errors.Wrap(txErr, "ERR_DB_DELETE", http.StatusInternalServerError)
	}
	d.publish(wsChannelGlobal, wsEventDeleted, map[string]any{
		"action":    wsActionDelete,
		"contactId": id,
	})
	return nil
}

func (d *Deps) List(ctx context.Context, searchParam string, pageNumber int) ([]Contact, int64, bool, *errors.AppError) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	q := d.DB.WithContext(ctx).Model(&Contact{})
	searchParam = strings.TrimSpace(searchParam)
	if searchParam != "" {
		pattern := "%" + escapeLike(strings.ToLower(searchParam)) + "%"
		q = q.Where("LOWER(name) ILIKE ? OR number LIKE ?", pattern, pattern)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	var items []Contact
	offset := (pageNumber - 1) * pageSize
	if err := q.Preload("ExtraInfo").Order("name ASC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	hasMore := count > int64(offset+len(items))
	return items, count, hasMore, nil
}

func (d *Deps) CountByNumber(ctx context.Context, number string) (int64, *errors.AppError) {
	normalized := NormalizeNumber(number, false)
	if normalized == "" {
		return 0, nil
	}
	var count int64
	if err := d.DB.WithContext(ctx).Model(&Contact{}).
		Where("number = ?", normalized).
		Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return count, nil
}

func (d *Deps) loadByID(ctx context.Context, id uint) (*Contact, *errors.AppError) {
	var entity Contact
	err := d.DB.WithContext(ctx).
		Preload("ExtraInfo").
		First(&entity, id).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errNoContactFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &entity, nil
}

func (d *Deps) numberTakenByOther(ctx context.Context, number string, id uint) (bool, *errors.AppError) {
	if number == "" {
		return false, nil
	}
	var count int64
	if err := d.DB.WithContext(ctx).Model(&Contact{}).
		Where("number = ? AND id != ?", number, id).
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

func findOneByField(tx *gorm.DB, column, value string) (*Contact, error) {
	if value == "" {
		return nil, nil
	}
	var entity Contact
	err := tx.Where(column+" = ?", value).First(&entity).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

func translateUniqueViolation(err error) *errors.AppError {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return nil
	}
	if pgErr.Code != pgUniqueViolation {
		return nil
	}
	return errors.New(errDuplicatedContact, http.StatusBadRequest)
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
