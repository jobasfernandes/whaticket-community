package message

import (
	"time"

	"github.com/jobasfernandes/whaticket-go-backend/internal/contact"
)

type Message struct {
	ID          string           `gorm:"primaryKey;type:varchar(255)"`
	TicketID    uint             `gorm:"column:ticket_id;not null;index:messages_ticket_created_idx,priority:1"`
	ContactID   *uint            `gorm:"column:contact_id"`
	Body        string           `gorm:"type:text;not null;default:''"`
	MediaType   string           `gorm:"column:media_type;size:50;not null;default:'chat'"`
	MediaURL    string           `gorm:"column:media_url;type:text;not null;default:''"`
	FromMe      bool             `gorm:"column:from_me;not null;default:false"`
	Read        bool             `gorm:"column:read;not null;default:false"`
	Ack         int              `gorm:"not null;default:0"`
	QuotedMsgID *string          `gorm:"column:quoted_msg_id;type:varchar(255);index:messages_quoted_idx"`
	QuotedMsg   *Message         `gorm:"foreignKey:QuotedMsgID;references:ID"`
	Contact     *contact.Contact `gorm:"foreignKey:ContactID;references:ID"`
	Ticket      *ticketRef       `gorm:"foreignKey:TicketID;references:ID"`
	CreatedAt   time.Time        `gorm:"column:created_at;not null;default:now();index:messages_ticket_created_idx,priority:2,sort:desc"`
	UpdatedAt   time.Time        `gorm:"column:updated_at;not null;default:now()"`
}

func (Message) TableName() string {
	return "messages"
}

type ticketRef struct {
	ID uint `gorm:"primaryKey"`
}

func (ticketRef) TableName() string {
	return "tickets"
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

type MessageDTO struct {
	ID          string              `json:"id"`
	TicketID    uint                `json:"ticketId"`
	ContactID   *uint               `json:"contactId"`
	Body        string              `json:"body"`
	MediaType   string              `json:"mediaType"`
	MediaURL    string              `json:"mediaUrl"`
	FromMe      bool                `json:"fromMe"`
	Read        bool                `json:"read"`
	Ack         int                 `json:"ack"`
	QuotedMsgID *string             `json:"quotedMsgId"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	Contact     *contact.ContactDTO `json:"contact"`
	QuotedMsg   *MessageDTO         `json:"quotedMsg"`
}

func Serialize(m *Message) MessageDTO {
	dto := MessageDTO{
		ID:          m.ID,
		TicketID:    m.TicketID,
		ContactID:   m.ContactID,
		Body:        m.Body,
		MediaType:   m.MediaType,
		MediaURL:    m.MediaURL,
		FromMe:      m.FromMe,
		Read:        m.Read,
		Ack:         m.Ack,
		QuotedMsgID: m.QuotedMsgID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	if m.Contact != nil {
		c := contact.Serialize(m.Contact)
		dto.Contact = &c
	}
	if m.QuotedMsg != nil {
		q := Serialize(m.QuotedMsg)
		dto.QuotedMsg = &q
	}
	return dto
}
