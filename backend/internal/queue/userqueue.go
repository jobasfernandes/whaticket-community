package queue

import "time"

type UserQueue struct {
	UserID    uint      `gorm:"primaryKey"`
	QueueID   uint      `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (UserQueue) TableName() string {
	return "user_queues"
}
