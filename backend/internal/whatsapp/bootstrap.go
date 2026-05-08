package whatsapp

import (
	"context"
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

func StartAllSessions(ctx context.Context, db *gorm.DB, pub RMQPublisher, log *slog.Logger) *errors.AppError {
	if log == nil {
		log = slog.Default()
	}
	var sessions []Whatsapp
	err := db.WithContext(ctx).
		Where("status != ?", StatusDisconnected).
		Order("id ASC").
		Find(&sessions).Error
	if err != nil {
		return errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	for i := range sessions {
		w := &sessions[i]
		if err := db.WithContext(ctx).Model(&Whatsapp{}).Where("id = ?", w.ID).Update("qrcode", "").Error; err != nil {
			log.Warn("clear stale qrcode failed",
				slog.Uint64("whatsapp_id", uint64(w.ID)),
				slog.Any("err", err),
			)
			continue
		}
		w.QRCode = ""

		if err := PublishStartSession(ctx, pub, w); err != nil {
			log.Warn("rmq publish session.start failed at bootstrap",
				slog.Uint64("whatsapp_id", uint64(w.ID)),
				slog.Any("err", err),
			)
			continue
		}
		log.Info("bootstrap published session.start",
			slog.Uint64("whatsapp_id", uint64(w.ID)),
			slog.String("status", w.Status),
		)
	}
	return nil
}
