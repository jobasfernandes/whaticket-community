package user

import (
	"context"
	stdErrors "errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
)

type AuthLoader struct {
	DB *gorm.DB
}

func (a *AuthLoader) FindByEmail(ctx context.Context, db *gorm.DB, email string) (*auth.UserRecord, error) {
	var u User
	if err := db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("user.AuthLoader.FindByEmail: %w", err)
	}
	return toAuthRecord(&u), nil
}

func (a *AuthLoader) FindByID(ctx context.Context, db *gorm.DB, id uint) (*auth.UserRecord, error) {
	var u User
	if err := db.WithContext(ctx).First(&u, id).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("user.AuthLoader.FindByID: %w", err)
	}
	return toAuthRecord(&u), nil
}

func (a *AuthLoader) Serialize(ctx context.Context, db *gorm.DB, record *auth.UserRecord) (auth.SerializedUser, error) {
	if record == nil {
		return auth.SerializedUser{}, fmt.Errorf("user.AuthLoader.Serialize: nil record")
	}
	var u User
	err := db.WithContext(ctx).
		Preload("Queues", func(tx *gorm.DB) *gorm.DB { return tx.Order("queues.name ASC") }).
		Preload("Whatsapp").
		First(&u, record.ID).Error
	if err != nil {
		return auth.SerializedUser{}, fmt.Errorf("user.AuthLoader.Serialize: %w", err)
	}
	dto := Serialize(&u)
	queues := make([]any, 0, len(dto.Queues))
	for _, q := range dto.Queues {
		queues = append(queues, q)
	}
	var whatsapp any
	if dto.Whatsapp != nil {
		whatsapp = dto.Whatsapp
	}
	return auth.SerializedUser{
		ID:       dto.ID,
		Name:     dto.Name,
		Email:    dto.Email,
		Profile:  dto.Profile,
		Queues:   queues,
		Whatsapp: whatsapp,
	}, nil
}

func toAuthRecord(u *User) *auth.UserRecord {
	return &auth.UserRecord{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Profile:      u.Profile,
		TokenVersion: u.TokenVersion,
	}
}
