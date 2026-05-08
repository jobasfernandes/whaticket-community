package message

import (
	"context"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

type TicketService interface {
	Show(ctx context.Context, ticketID uint, actor *auth.UserClaims) (TicketLike, *errors.AppError)
	LoadByID(ctx context.Context, ticketID uint) (TicketLike, *errors.AppError)
	UpdateLastMessage(ctx context.Context, ticketID uint, body string) *errors.AppError
	SerializeTicket(t TicketLike) any
}

type TicketLike interface {
	GetID() uint
	GetStatus() string
	GetUserID() *uint
	GetUnreadMessages() int
	GetWhatsappID() uint
	GetContactID() uint
}

type WSPublisher interface {
	Publish(channel, event string, data any)
}
