package user

import (
	"context"
	stdErrors "errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
)

const (
	pageSize                = 20
	wsChannelGlobal         = "global"
	wsEventCreated          = "user.created"
	wsEventUpdated          = "user.updated"
	wsEventDeleted          = "user.deleted"
	wsActionCreate          = "create"
	wsActionUpdate          = "update"
	wsActionDelete          = "delete"
	pgUniqueViolationCode   = "23505"
	pgFKViolationCode       = "23503"
	settingKeyUserCreation  = "userCreation"
	settingValueEnabled     = "enabled"
	defaultProfile          = "admin"
	errNoPermission         = "ERR_NO_PERMISSION"
	errNoUserFound          = "ERR_NO_USER_FOUND"
	errUserEmailExists      = "ERR_USER_EMAIL_EXISTS"
	errUserCreationDisabled = "ERR_USER_CREATION_DISABLED"
	errInvalidWhatsapp      = "ERR_INVALID_WHATSAPP"
)

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type SettingChecker interface {
	Check(ctx context.Context, db *gorm.DB, key string) (string, *errors.AppError)
}

type Deps struct {
	DB       *gorm.DB
	WS       WSPublisher
	Settings SettingChecker
}

func (d *Deps) Create(ctx context.Context, req CreateRequest, isSignup, actorIsAdmin bool) (*User, *errors.AppError) {
	if isSignup {
		value, err := d.Settings.Check(ctx, d.DB, settingKeyUserCreation)
		if err != nil {
			return nil, err
		}
		if value != settingValueEnabled {
			return nil, errors.New(errUserCreationDisabled, http.StatusForbidden)
		}
	} else if !actorIsAdmin {
		return nil, errors.New(errNoPermission, http.StatusForbidden)
	}

	if appErr := validateCreate(&req); appErr != nil {
		return nil, appErr
	}

	profile := req.Profile
	if profile == "" {
		profile = defaultProfile
	}

	entity := &User{
		Name:         req.Name,
		Email:        req.Email,
		Password:     req.Password,
		Profile:      profile,
		WhatsappID:   req.WhatsappID,
		TokenVersion: 0,
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		if req.QueueIDs != nil {
			refs := buildQueueRefs(req.QueueIDs)
			if err := tx.Model(entity).Association("Queues").Replace(refs); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		if appErr := translatePgError(txErr); appErr != nil {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_INSERT", http.StatusInternalServerError)
	}

	loaded, appErr := d.loadByID(ctx, entity.ID)
	if appErr != nil {
		return nil, appErr
	}

	d.publish(wsChannelGlobal, wsEventCreated, map[string]any{
		"action": wsActionCreate,
		"user":   Serialize(loaded),
	})
	return loaded, nil
}

func (d *Deps) Show(ctx context.Context, id uint) (*User, *errors.AppError) {
	return d.loadByID(ctx, id)
}

func (d *Deps) List(ctx context.Context, searchParam string, pageNumber int) ([]User, int64, bool, *errors.AppError) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	q := d.DB.WithContext(ctx).Model(&User{})
	searchParam = strings.TrimSpace(searchParam)
	if searchParam != "" {
		pattern := "%" + escapeLike(strings.ToLower(searchParam)) + "%"
		q = q.Where("LOWER(name) ILIKE ? OR LOWER(email) ILIKE ?", pattern, pattern)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	var items []User
	offset := (pageNumber - 1) * pageSize
	if err := q.Preload("Queues").Preload("Whatsapp").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	hasMore := count > int64(offset+len(items))
	return items, count, hasMore, nil
}

func (d *Deps) Update(ctx context.Context, id uint, req UpdateRequest, actorIsAdmin bool) (*User, *errors.AppError) {
	if !actorIsAdmin {
		return nil, errors.New(errNoPermission, http.StatusForbidden)
	}
	if appErr := validateUpdate(&req); appErr != nil {
		return nil, appErr
	}

	var existing User
	if err := d.DB.WithContext(ctx).First(&existing, id).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errNoUserFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Profile != nil {
		updates["profile"] = *req.Profile
	}
	if req.WhatsappID != nil {
		if *req.WhatsappID == nil {
			updates["whatsapp_id"] = gorm.Expr("NULL")
		} else {
			updates["whatsapp_id"] = **req.WhatsappID
		}
	}
	if req.Password != nil && *req.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, errors.Wrap(err, "ERR_HASH_PASSWORD", http.StatusInternalServerError)
		}
		updates["password_hash"] = string(hashed)
		updates["token_version"] = gorm.Expr("token_version + 1")
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if req.QueueIDs != nil {
			refs := buildQueueRefs(*req.QueueIDs)
			if err := tx.Model(&User{ID: id}).Association("Queues").Replace(refs); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		if appErr := translatePgError(txErr); appErr != nil {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}

	loaded, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	d.publish(wsChannelGlobal, wsEventUpdated, map[string]any{
		"action": wsActionUpdate,
		"user":   Serialize(loaded),
	})
	return loaded, nil
}

func (d *Deps) Delete(ctx context.Context, id uint, actorIsAdmin bool) *errors.AppError {
	if !actorIsAdmin {
		return errors.New(errNoPermission, http.StatusForbidden)
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE tickets SET user_id = NULL, status = 'pending' WHERE user_id = ? AND status = 'open'", id).Error; err != nil {
			return err
		}
		res := tx.Delete(&User{}, id)
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
			return errors.New(errNoUserFound, http.StatusNotFound)
		}
		return errors.Wrap(txErr, "ERR_DB_DELETE", http.StatusInternalServerError)
	}

	d.publish(wsChannelGlobal, wsEventDeleted, map[string]any{
		"action": wsActionDelete,
		"userId": id,
	})
	return nil
}

func (d *Deps) loadByID(ctx context.Context, id uint) (*User, *errors.AppError) {
	var u User
	err := d.DB.WithContext(ctx).
		Preload("Queues", func(tx *gorm.DB) *gorm.DB { return tx.Order("queues.name ASC") }).
		Preload("Whatsapp").
		First(&u, id).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errNoUserFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &u, nil
}

func (d *Deps) publish(channel, event string, data any) {
	if d.WS == nil {
		return
	}
	d.WS.Publish(channel, event, data)
}

func buildQueueRefs(ids []uint) []queue.Queue {
	refs := make([]queue.Queue, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, queue.Queue{ID: id})
	}
	return refs
}

func translatePgError(err error) *errors.AppError {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return nil
	}
	switch pgErr.Code {
	case pgUniqueViolationCode:
		if strings.Contains(strings.ToLower(pgErr.ConstraintName), "email") {
			return errors.New(errUserEmailExists, http.StatusBadRequest)
		}
	case pgFKViolationCode:
		name := strings.ToLower(pgErr.ConstraintName)
		if strings.Contains(name, "queue") {
			return errors.New(errInvalidQueue, http.StatusBadRequest)
		}
		if strings.Contains(name, "whatsapp") {
			return errors.New(errInvalidWhatsapp, http.StatusBadRequest)
		}
	}
	return nil
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
