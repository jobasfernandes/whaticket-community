package ticket

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
)

type WSAuthorizer struct {
	Deps *Deps
}

func NewWSAuthz(deps *Deps) *WSAuthorizer {
	return &WSAuthorizer{Deps: deps}
}

func (a *WSAuthorizer) CanSee(ctx context.Context, userID uint, profile string, ticketID uint) (bool, error) {
	var t Ticket
	err := a.Deps.DB.WithContext(ctx).First(&t, ticketID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	queues, qerr := a.Deps.UserService.GetQueueIDs(ctx, userID)
	if qerr != nil {
		return false, qerr
	}
	claims := &auth.UserClaims{ID: userID, Profile: profile}
	return canSee(claims, &t, queues), nil
}
