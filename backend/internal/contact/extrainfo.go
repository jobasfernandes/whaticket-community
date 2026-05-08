package contact

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

func upsertExtraInfo(tx *gorm.DB, contactID uint, payload []CustomFieldData) *errors.AppError {
	if appErr := validateExtraInfo(payload); appErr != nil {
		return appErr
	}

	var existing []ContactCustomField
	if err := tx.Where("contact_id = ?", contactID).Find(&existing).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	payloadIDs := make(map[uint]CustomFieldData, len(payload))
	for i := range payload {
		if payload[i].ID != nil {
			payloadIDs[*payload[i].ID] = payload[i]
		}
	}

	idsToDelete := make([]uint, 0)
	for i := range existing {
		if _, kept := payloadIDs[existing[i].ID]; !kept {
			idsToDelete = append(idsToDelete, existing[i].ID)
		}
	}
	if len(idsToDelete) > 0 {
		if err := tx.Where("id IN ?", idsToDelete).Delete(&ContactCustomField{}).Error; err != nil {
			return errors.Wrap(err, "ERR_DB_DELETE", http.StatusInternalServerError)
		}
	}

	for id, item := range payloadIDs {
		if err := tx.Model(&ContactCustomField{}).
			Where("id = ? AND contact_id = ?", id, contactID).
			Updates(map[string]any{"name": item.Name, "value": item.Value}).Error; err != nil {
			return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
		}
	}

	for i := range payload {
		if payload[i].ID != nil {
			continue
		}
		row := &ContactCustomField{
			ContactID: contactID,
			Name:      payload[i].Name,
			Value:     payload[i].Value,
		}
		if err := tx.Create(row).Error; err != nil {
			return errors.Wrap(err, "ERR_DB_INSERT", http.StatusInternalServerError)
		}
	}

	return nil
}
