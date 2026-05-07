package ticket

import (
	"context"

	"gorm.io/gorm"
)

func withContactLock(ctx context.Context, db *gorm.DB, contactID, whatsappID uint, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?::int4, ?::int4)", contactID, whatsappID).Error; err != nil {
			return err
		}
		return fn(tx)
	})
}
