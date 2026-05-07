package whatsapp

import (
	"time"

	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
)

const (
	StatusOpening      = "OPENING"
	StatusQRCode       = "qrcode"
	StatusConnected    = "CONNECTED"
	StatusDisconnected = "DISCONNECTED"
)

const (
	MediaDeliveryBase64 = "base64"
	MediaDeliveryS3     = "s3"
	MediaDeliveryBoth   = "both"
	MediaDeliveryURL    = "url"
)

type AdvancedSettings struct {
	AlwaysOnline  bool   `gorm:"column:always_online;not null;default:false" json:"alwaysOnline"`
	RejectCall    bool   `gorm:"column:reject_call;not null;default:false" json:"rejectCall"`
	MsgRejectCall string `gorm:"column:msg_reject_call;type:text;not null;default:''" json:"msgRejectCall"`
	ReadMessages  bool   `gorm:"column:read_messages;not null;default:false" json:"readMessages"`
	IgnoreGroups  bool   `gorm:"column:ignore_groups;not null;default:false" json:"ignoreGroups"`
	IgnoreStatus  bool   `gorm:"column:ignore_status;not null;default:false" json:"ignoreStatus"`
}

type Whatsapp struct {
	ID               uint             `gorm:"primaryKey;autoIncrement"`
	Name             string           `gorm:"size:255;not null;uniqueIndex:whatsapps_name_uniq"`
	Status           string           `gorm:"size:50;not null;default:'OPENING'"`
	QRCode           string           `gorm:"column:qrcode;type:text;not null;default:''"`
	Session          string           `gorm:"column:session;type:text" json:"-"`
	Battery          string           `gorm:"column:battery;size:50" json:"-"`
	Plugged          bool             `gorm:"column:plugged" json:"-"`
	Retries          int              `gorm:"column:retries;not null;default:0"`
	IsDefault        bool             `gorm:"column:is_default;not null;default:false"`
	GreetingMessage  string           `gorm:"column:greeting_message;type:text;not null;default:''"`
	FarewellMessage  string           `gorm:"column:farewell_message;type:text;not null;default:''"`
	AdvancedSettings AdvancedSettings `gorm:"embedded"`
	MediaDelivery    string           `gorm:"column:media_delivery;size:20;not null;default:'base64'"`
	Queues           []queue.Queue    `gorm:"many2many:whatsapp_queues;"`
	CreatedAt        time.Time        `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt        time.Time        `gorm:"column:updated_at;not null;default:now()"`
}

func (Whatsapp) TableName() string {
	return "whatsapps"
}

type QueueBriefDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type WhatsappDTO struct {
	ID               uint             `json:"id"`
	Name             string           `json:"name"`
	Status           string           `json:"status"`
	QRCode           string           `json:"qrcode"`
	Retries          int              `json:"retries"`
	GreetingMessage  string           `json:"greetingMessage"`
	FarewellMessage  string           `json:"farewellMessage"`
	IsDefault        bool             `json:"isDefault"`
	AdvancedSettings AdvancedSettings `json:"advancedSettings"`
	MediaDelivery    string           `json:"mediaDelivery"`
	Queues           []QueueBriefDTO  `json:"queues"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

func Serialize(w *Whatsapp) WhatsappDTO {
	queues := make([]QueueBriefDTO, 0, len(w.Queues))
	for i := range w.Queues {
		queues = append(queues, QueueBriefDTO{
			ID:    w.Queues[i].ID,
			Name:  w.Queues[i].Name,
			Color: w.Queues[i].Color,
		})
	}
	mediaDelivery := w.MediaDelivery
	if mediaDelivery == "" {
		mediaDelivery = MediaDeliveryBase64
	}
	return WhatsappDTO{
		ID:               w.ID,
		Name:             w.Name,
		Status:           w.Status,
		QRCode:           w.QRCode,
		Retries:          w.Retries,
		GreetingMessage:  w.GreetingMessage,
		FarewellMessage:  w.FarewellMessage,
		IsDefault:        w.IsDefault,
		AdvancedSettings: w.AdvancedSettings,
		MediaDelivery:    mediaDelivery,
		Queues:           queues,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
	}
}
