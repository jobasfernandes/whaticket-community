package contact

import (
	stdErrors "errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	actionCreated       = "created"
	actionUpdatedSame   = "updated_same"
	actionMerged        = "merged"
	actionUpdatedLID    = "updated_lid"
	actionUpdatedNumber = "updated_number"
	pgUndefinedTable    = "42P01"
)

func merge(tx *gorm.DB, byNumber, byLID *Contact, req CreateOrUpdateRequest) (*Contact, string, error) {
	switch {
	case byNumber == nil && byLID == nil:
		return mergeInsertNew(tx, req)
	case byNumber != nil && byLID != nil && byNumber.ID == byLID.ID:
		return mergeUpdateSame(tx, byNumber, req)
	case byNumber != nil && byLID != nil && byNumber.ID != byLID.ID:
		return mergeReassignAndDestroy(tx, byNumber, byLID, req)
	case byNumber != nil:
		return mergeUpdateLID(tx, byNumber, req)
	default:
		return mergeUpdateNumber(tx, byLID, req)
	}
}

func mergeInsertNew(tx *gorm.DB, req CreateOrUpdateRequest) (*Contact, string, error) {
	entity := &Contact{
		Name:          req.Name,
		Number:        req.Number,
		LID:           req.LID,
		Email:         req.Email,
		ProfilePicURL: req.ProfilePicURL,
		IsGroup:       req.IsGroup,
	}
	if err := tx.Create(entity).Error; err != nil {
		return nil, "", err
	}
	return entity, actionCreated, nil
}

func mergeUpdateSame(tx *gorm.DB, existing *Contact, req CreateOrUpdateRequest) (*Contact, string, error) {
	updates := updatesFromReq(existing, req)
	if len(updates) == 0 {
		return existing, actionUpdatedSame, nil
	}
	if err := tx.Model(&Contact{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return nil, "", err
	}
	return existing, actionUpdatedSame, nil
}

func mergeReassignAndDestroy(tx *gorm.DB, byNumber, byLID *Contact, req CreateOrUpdateRequest) (*Contact, string, error) {
	if err := execTolerateMissing(tx, "UPDATE tickets SET contact_id = ? WHERE contact_id = ?", byNumber.ID, byLID.ID); err != nil {
		return nil, "", err
	}
	if err := execTolerateMissing(tx, "UPDATE messages SET contact_id = ? WHERE contact_id = ?", byNumber.ID, byLID.ID); err != nil {
		return nil, "", err
	}
	updates := updatesFromReq(byNumber, req)
	updates["lid"] = req.LID
	if v, ok := updates["profile_pic_url"]; !ok || v == "" {
		if req.ProfilePicURL != "" {
			updates["profile_pic_url"] = req.ProfilePicURL
		}
	}
	if err := tx.Model(&Contact{}).Where("id = ?", byNumber.ID).Updates(updates).Error; err != nil {
		return nil, "", err
	}
	if err := tx.Delete(&Contact{}, byLID.ID).Error; err != nil {
		return nil, "", err
	}
	return byNumber, actionMerged, nil
}

func mergeUpdateLID(tx *gorm.DB, existing *Contact, req CreateOrUpdateRequest) (*Contact, string, error) {
	updates := updatesFromReq(existing, req)
	if req.LID != "" {
		updates["lid"] = req.LID
	}
	if len(updates) == 0 {
		return existing, actionUpdatedLID, nil
	}
	if err := tx.Model(&Contact{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return nil, "", err
	}
	return existing, actionUpdatedLID, nil
}

func mergeUpdateNumber(tx *gorm.DB, existing *Contact, req CreateOrUpdateRequest) (*Contact, string, error) {
	updates := updatesFromReq(existing, req)
	if req.Number != "" {
		updates["number"] = req.Number
	}
	if len(updates) == 0 {
		return existing, actionUpdatedNumber, nil
	}
	if err := tx.Model(&Contact{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return nil, "", err
	}
	return existing, actionUpdatedNumber, nil
}

func updatesFromReq(existing *Contact, req CreateOrUpdateRequest) map[string]any {
	updates := map[string]any{}
	if name := keepIfEmpty(existing.Name, req.Name); name != existing.Name {
		updates["name"] = name
	}
	if pic := keepIfEmpty(existing.ProfilePicURL, req.ProfilePicURL); pic != existing.ProfilePicURL {
		updates["profile_pic_url"] = pic
	}
	if req.Email != "" && req.Email != existing.Email {
		updates["email"] = req.Email
	}
	return updates
}

func keepIfEmpty(existing, incoming string) string {
	if incoming == "" {
		return existing
	}
	return incoming
}

func execTolerateMissing(tx *gorm.DB, sql string, args ...any) error {
	if err := tx.Exec(sql, args...).Error; err != nil {
		if isMissingTableError(err) {
			return nil
		}
		return err
	}
	return nil
}

func isMissingTableError(err error) bool {
	var pgErr *pgconn.PgError
	if !stdErrors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgUndefinedTable
}
