package waevents

import "context"

type WhatsappService interface {
	Show(ctx context.Context, id uint) (Whatsapp, error)
	PublishStopSession(ctx context.Context, whatsappID uint) error
	PublishLogout(ctx context.Context, whatsappID uint) error
	UpdateStatus(ctx context.Context, id uint, status, qrcode string) error
	UpdateConnected(ctx context.Context, id uint) error
	UpdateRetries(ctx context.Context, id uint) error
	UpdateDisconnected(ctx context.Context, id uint) error
	SerializeForWS(w Whatsapp) any
}

type Whatsapp interface {
	GetID() uint
	GetStatus() string
	GetGreetingMessage() string
	GetFarewellMessage() string
	GetQueues() []QueueLike
}

type QueueLike interface {
	GetID() uint
	GetName() string
	GetGreetingMessage() string
}

type ContactService interface {
	CreateOrUpdate(ctx context.Context, number, name, lid string, isGroup bool) (Contact, error)
	Create(ctx context.Context, name, number string) error
}

type Contact interface {
	GetID() uint
	GetNumber() string
	GetName() string
}

type TicketService interface {
	FindOrCreate(ctx context.Context, contact Contact, whatsappID uint, unreadMessages int, groupContact Contact) (Ticket, error)
	UpdateLastMessage(ctx context.Context, ticketID uint, body string) error
	UpdateQueue(ctx context.Context, ticketID uint, queueID uint) error
	SetMessagesAsRead(ctx context.Context, ticketID uint) error
}

type Ticket interface {
	GetID() uint
	GetStatus() string
	GetUserID() *uint
	GetQueueID() *uint
	GetUnreadMessages() int
}

type MessageService interface {
	Create(ctx context.Context, data MessageData) error
}

type MessageData struct {
	ID          string
	TicketID    uint
	ContactID   *uint
	Body        string
	FromMe      bool
	Read        bool
	MediaType   string
	MediaURL    string
	QuotedMsgID *string
	Ack         int
}

type WSPublisher interface {
	Publish(channel, event string, data any)
}

type RMQPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, env any) error
}

type RPCClient interface {
	Call(ctx context.Context, exchange, routingKey string, req any, resp any) error
}
