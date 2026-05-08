package queue

import "time"

type WhatsappQueue struct {
	WhatsappID uint      `gorm:"primaryKey"`
	QueueID    uint      `gorm:"primaryKey"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
	UpdatedAt  time.Time `gorm:"not null;default:now()"`
}

func (WhatsappQueue) TableName() string {
	return "whatsapp_queues"
}
