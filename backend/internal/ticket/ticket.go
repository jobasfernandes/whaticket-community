package ticket

import (
	"time"

	"github.com/canove/whaticket-community/backend/internal/contact"
	"github.com/canove/whaticket-community/backend/internal/queue"
	"github.com/canove/whaticket-community/backend/internal/user"
	"github.com/canove/whaticket-community/backend/internal/whatsapp"
)

type Ticket struct {
	ID             uint              `gorm:"primaryKey;autoIncrement"`
	Status         string            `gorm:"size:20;not null;default:'pending'"`
	UnreadMessages int               `gorm:"column:unread_messages;not null;default:0"`
	LastMessage    string            `gorm:"column:last_message;type:text;not null;default:''"`
	IsGroup        bool              `gorm:"column:is_group;not null;default:false"`
	UserID         *uint             `gorm:"column:user_id"`
	ContactID      uint              `gorm:"column:contact_id;not null"`
	WhatsappID     uint              `gorm:"column:whatsapp_id;not null"`
	QueueID        *uint             `gorm:"column:queue_id"`
	Contact        contact.Contact   `gorm:"foreignKey:ContactID;references:ID"`
	User           *user.User        `gorm:"foreignKey:UserID;references:ID"`
	Queue          *queue.Queue      `gorm:"foreignKey:QueueID;references:ID"`
	Whatsapp       whatsapp.Whatsapp `gorm:"foreignKey:WhatsappID;references:ID"`
	CreatedAt      time.Time         `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt      time.Time         `gorm:"column:updated_at;not null;default:now()"`
}

func (Ticket) TableName() string {
	return "tickets"
}

func (t *Ticket) GetID() uint            { return t.ID }
func (t *Ticket) GetStatus() string      { return t.Status }
func (t *Ticket) GetUserID() *uint       { return t.UserID }
func (t *Ticket) GetQueueID() *uint      { return t.QueueID }
func (t *Ticket) GetUnreadMessages() int { return t.UnreadMessages }
func (t *Ticket) GetWhatsappID() uint    { return t.WhatsappID }
func (t *Ticket) GetContactID() uint     { return t.ContactID }

type UpdateData struct {
	Status         *string
	LastMessage    *string
	UserID         **uint
	QueueID        **uint
	UnreadMessages *int
}

type UserBriefDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type QueueBriefDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type TicketWhatsappDTO struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type TicketDTO struct {
	ID             uint               `json:"id"`
	Status         string             `json:"status"`
	UnreadMessages int                `json:"unreadMessages"`
	LastMessage    string             `json:"lastMessage"`
	IsGroup        bool               `json:"isGroup"`
	UserID         *uint              `json:"userId"`
	QueueID        *uint              `json:"queueId"`
	WhatsappID     uint               `json:"whatsappId"`
	ContactID      uint               `json:"contactId"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	Contact        contact.ContactDTO `json:"contact"`
	User           *UserBriefDTO      `json:"user"`
	Queue          *QueueBriefDTO     `json:"queue"`
	Whatsapp       TicketWhatsappDTO  `json:"whatsapp"`
}

func Serialize(t *Ticket) TicketDTO {
	dto := TicketDTO{
		ID:             t.ID,
		Status:         t.Status,
		UnreadMessages: t.UnreadMessages,
		LastMessage:    t.LastMessage,
		IsGroup:        t.IsGroup,
		UserID:         t.UserID,
		QueueID:        t.QueueID,
		WhatsappID:     t.WhatsappID,
		ContactID:      t.ContactID,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
		Contact:        contact.Serialize(&t.Contact),
		Whatsapp: TicketWhatsappDTO{
			ID:     t.Whatsapp.ID,
			Name:   t.Whatsapp.Name,
			Status: t.Whatsapp.Status,
		},
	}
	if t.User != nil {
		dto.User = &UserBriefDTO{
			ID:    t.User.ID,
			Name:  t.User.Name,
			Email: t.User.Email,
		}
	}
	if t.Queue != nil {
		dto.Queue = &QueueBriefDTO{
			ID:    t.Queue.ID,
			Name:  t.Queue.Name,
			Color: t.Queue.Color,
		}
	}
	return dto
}
