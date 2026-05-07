package whatsapp

import (
	"context"
	stdErrors "errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
	"github.com/jobasfernandes/whaticket-go-backend/internal/rmq"
)

const (
	wsChannelNotification       = "notification"
	wsEventSessionUpdate        = "whatsappSession.update"
	wsEventSessionPairPhone     = "whatsappSession.pairphone"
	pgUniqueViolationCode       = "23505"
	whatsappNameUniqueIndex     = "whatsapps_name_uniq"
	whatsappOneDefaultUniqueIdx = "whatsapps_one_default"
	errWhatsappNotFound         = "ERR_WAPP_NOT_FOUND"
	errWhatsappNameExists       = "ERR_WAPP_NAME_EXISTS"
	errNoDefaultWhatsapp        = "ERR_NO_DEFAULT_WHATSAPP"
	errInvalidQueueAssociation  = "ERR_INVALID_QUEUE"
	deleteActionLabel           = "delete"
)

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type RMQPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, env rmq.Envelope) error
}

type Deps struct {
	DB     *gorm.DB
	WS     WSPublisher
	RMQ    RMQPublisher
	RPC    RPCClient
	Logger *slog.Logger
}

func (d *Deps) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}

func (d *Deps) Create(ctx context.Context, req CreateRequest) (*Whatsapp, *errors.AppError) {
	trimCreate(&req)
	if appErr := validateCreate(&req); appErr != nil {
		return nil, appErr
	}

	mediaDelivery := req.MediaDelivery
	if mediaDelivery == "" {
		mediaDelivery = MediaDeliveryBase64
	}

	entity := &Whatsapp{
		Name:            req.Name,
		Status:          StatusOpening,
		QRCode:          "",
		Retries:         0,
		IsDefault:       req.IsDefault != nil && *req.IsDefault,
		GreetingMessage: req.GreetingMessage,
		FarewellMessage: req.FarewellMessage,
		MediaDelivery:   mediaDelivery,
	}
	if req.AdvancedSettings != nil {
		entity.AdvancedSettings = *req.AdvancedSettings
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entity.IsDefault {
			if err := tx.Exec("UPDATE whatsapps SET is_default = false WHERE is_default = true").Error; err != nil {
				return err
			}
		}
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		if len(req.QueueIDs) > 0 {
			queues, err := loadQueuesByIDs(tx, req.QueueIDs)
			if err != nil {
				return err
			}
			if err := tx.Model(entity).Association("Queues").Replace(queues); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		if appErr := translatePgError(txErr); appErr != nil {
			return nil, appErr
		}
		var appErr *errors.AppError
		if stdErrors.As(txErr, &appErr) {
			return nil, appErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_INSERT", http.StatusInternalServerError)
	}

	loaded, appErr := d.loadByID(ctx, entity.ID)
	if appErr != nil {
		return nil, appErr
	}

	if err := PublishStartSession(ctx, d.RMQ, loaded); err != nil {
		d.logger().Warn("rmq publish session.start failed",
			slog.Uint64("whatsapp_id", uint64(loaded.ID)),
			slog.Any("err", err),
		)
	}

	d.publish(wsChannelNotification, wsEventSessionUpdate, Serialize(loaded))
	return loaded, nil
}

func (d *Deps) Show(ctx context.Context, id uint) (*Whatsapp, *errors.AppError) {
	return d.loadByID(ctx, id)
}

func (d *Deps) List(ctx context.Context) ([]Whatsapp, *errors.AppError) {
	var items []Whatsapp
	if err := d.DB.WithContext(ctx).
		Preload("Queues").
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return items, nil
}

func (d *Deps) Update(ctx context.Context, id uint, req UpdateRequest) (*Whatsapp, *errors.AppError) {
	trimUpdate(&req)
	if appErr := validateUpdate(&req); appErr != nil {
		return nil, appErr
	}

	existing, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	updates := buildUpdateMap(&req)
	settingsChanged := req.AdvancedSettings != nil || req.MediaDelivery != nil

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.IsDefault != nil && *req.IsDefault {
			if err := tx.Exec("UPDATE whatsapps SET is_default = false WHERE is_default = true AND id != ?", id).Error; err != nil {
				return err
			}
		}
		if len(updates) > 0 {
			if err := tx.Model(&Whatsapp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if req.QueueIDs != nil {
			queues, err := loadQueuesByIDs(tx, *req.QueueIDs)
			if err != nil {
				return err
			}
			if err := tx.Model(&Whatsapp{ID: id}).Association("Queues").Replace(queues); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		if appErr := translatePgError(txErr); appErr != nil {
			return nil, appErr
		}
		var asErr *errors.AppError
		if stdErrors.As(txErr, &asErr) {
			return nil, asErr
		}
		return nil, errors.Wrap(txErr, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}

	loaded, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	if settingsChanged && (loaded.AdvancedSettings != existing.AdvancedSettings || loaded.MediaDelivery != existing.MediaDelivery) {
		if err := PublishUpdateSettings(ctx, d.RMQ, loaded); err != nil {
			d.logger().Warn("rmq publish session.update_settings failed",
				slog.Uint64("whatsapp_id", uint64(loaded.ID)),
				slog.Any("err", err),
			)
		}
	}

	d.publish(wsChannelNotification, wsEventSessionUpdate, Serialize(loaded))
	return loaded, nil
}

func (d *Deps) Delete(ctx context.Context, id uint) *errors.AppError {
	loaded, appErr := d.loadByID(ctx, id)
	if appErr != nil {
		return appErr
	}

	if err := PublishLogout(ctx, d.RMQ, loaded.ID); err != nil {
		d.logger().Warn("rmq publish session.logout failed",
			slog.Uint64("whatsapp_id", uint64(loaded.ID)),
			slog.Any("err", err),
		)
	}

	txErr := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Whatsapp{ID: loaded.ID}).Association("Queues").Clear(); err != nil {
			return err
		}
		res := tx.Delete(&Whatsapp{}, loaded.ID)
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
			return errors.New(errWhatsappNotFound, http.StatusNotFound)
		}
		return errors.Wrap(txErr, "ERR_DB_DELETE", http.StatusInternalServerError)
	}

	d.publish(wsChannelNotification, wsEventSessionUpdate, map[string]any{
		"action":  deleteActionLabel,
		"session": Serialize(loaded),
	})
	return nil
}

func (d *Deps) loadByID(ctx context.Context, id uint) (*Whatsapp, *errors.AppError) {
	return Get(ctx, d.DB, id)
}

func (d *Deps) publish(channel, event string, data any) {
	if d.WS == nil {
		return
	}
	d.WS.Publish(channel, event, data)
}

func (d *Deps) UpdateStatus(ctx context.Context, id uint, status, qrcode string) *errors.AppError {
	updates := map[string]any{"status": status, "qrcode": qrcode}
	if err := d.DB.WithContext(ctx).Model(&Whatsapp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	return nil
}

func (d *Deps) UpdateConnected(ctx context.Context, id uint) *errors.AppError {
	updates := map[string]any{
		"status":  StatusConnected,
		"qrcode":  "",
		"retries": 0,
	}
	if err := d.DB.WithContext(ctx).Model(&Whatsapp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	return nil
}

func (d *Deps) UpdateRetries(ctx context.Context, id uint) *errors.AppError {
	if err := d.DB.WithContext(ctx).Model(&Whatsapp{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":  StatusOpening,
			"retries": gorm.Expr("retries + 1"),
		}).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	return nil
}

func (d *Deps) UpdateDisconnected(ctx context.Context, id uint) *errors.AppError {
	updates := map[string]any{
		"status":  StatusDisconnected,
		"qrcode":  "",
		"session": gorm.Expr("NULL"),
	}
	if err := d.DB.WithContext(ctx).Model(&Whatsapp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}
	return nil
}

func Get(ctx context.Context, db *gorm.DB, id uint) (*Whatsapp, *errors.AppError) {
	var w Whatsapp
	err := db.WithContext(ctx).
		Preload("Queues", func(tx *gorm.DB) *gorm.DB { return tx.Order("queues.name ASC") }).
		First(&w, id).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errWhatsappNotFound, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &w, nil
}

func GetDefault(ctx context.Context, db *gorm.DB) (*Whatsapp, *errors.AppError) {
	var w Whatsapp
	err := db.WithContext(ctx).
		Preload("Queues", func(tx *gorm.DB) *gorm.DB { return tx.Order("queues.name ASC") }).
		Where("is_default = ?", true).
		First(&w).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errNoDefaultWhatsapp, http.StatusNotFound)
		}
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	return &w, nil
}

func GetDefaultByUser(ctx context.Context, db *gorm.DB, userID uint) (*Whatsapp, *errors.AppError) {
	var assigned struct {
		WhatsappID *uint `gorm:"column:whatsapp_id"`
	}
	err := db.WithContext(ctx).
		Table("users").
		Select("whatsapp_id").
		Where("id = ?", userID).
		Take(&assigned).Error
	if err != nil && !stdErrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	if assigned.WhatsappID != nil && *assigned.WhatsappID > 0 {
		return Get(ctx, db, *assigned.WhatsappID)
	}
	return GetDefault(ctx, db)
}

func loadQueuesByIDs(tx *gorm.DB, ids []uint) ([]queue.Queue, error) {
	if len(ids) == 0 {
		return []queue.Queue{}, nil
	}
	deduped := dedupeIDs(ids)
	var queues []queue.Queue
	if err := tx.Where("id IN ?", deduped).Find(&queues).Error; err != nil {
		return nil, err
	}
	if len(queues) != len(deduped) {
		return nil, errors.New(errInvalidQueueAssociation, http.StatusBadRequest)
	}
	return queues, nil
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

func buildUpdateMap(req *UpdateRequest) map[string]any {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.GreetingMessage != nil {
		updates["greeting_message"] = *req.GreetingMessage
	}
	if req.FarewellMessage != nil {
		updates["farewell_message"] = *req.FarewellMessage
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if req.MediaDelivery != nil {
		updates["media_delivery"] = *req.MediaDelivery
	}
	if req.AdvancedSettings != nil {
		updates["always_online"] = req.AdvancedSettings.AlwaysOnline
		updates["reject_call"] = req.AdvancedSettings.RejectCall
		updates["msg_reject_call"] = req.AdvancedSettings.MsgRejectCall
		updates["read_messages"] = req.AdvancedSettings.ReadMessages
		updates["ignore_groups"] = req.AdvancedSettings.IgnoreGroups
		updates["ignore_status"] = req.AdvancedSettings.IgnoreStatus
	}
	return updates
}

func translatePgError(err error) *errors.AppError {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return nil
	}
	if pgErr.Code != pgUniqueViolationCode {
		return nil
	}
	switch pgErr.ConstraintName {
	case whatsappNameUniqueIndex:
		return errors.New(errWhatsappNameExists, http.StatusConflict)
	case whatsappOneDefaultUniqueIdx:
		return errors.New(errWhatsappNameExists, http.StatusConflict)
	}
	return nil
}
