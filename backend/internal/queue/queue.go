package queue

import "time"

type Queue struct {
	ID              uint      `gorm:"primaryKey;autoIncrement"`
	Name            string    `gorm:"size:255;not null;uniqueIndex:queues_name_uniq"`
	Color           string    `gorm:"size:20;not null;uniqueIndex:queues_color_uniq"`
	GreetingMessage string    `gorm:"type:text;not null;default:''"`
	CreatedAt       time.Time `gorm:"not null;default:now()"`
	UpdatedAt       time.Time `gorm:"not null;default:now()"`
}

func (Queue) TableName() string {
	return "queues"
}

type QueueDTO struct {
	ID              uint      `json:"id"`
	Name            string    `json:"name"`
	Color           string    `json:"color"`
	GreetingMessage string    `json:"greetingMessage"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func Serialize(q *Queue) QueueDTO {
	return QueueDTO{
		ID:              q.ID,
		Name:            q.Name,
		Color:           q.Color,
		GreetingMessage: q.GreetingMessage,
		CreatedAt:       q.CreatedAt,
		UpdatedAt:       q.UpdatedAt,
	}
}

type CreateRequest struct {
	Name            string `json:"name"`
	Color           string `json:"color"`
	GreetingMessage string `json:"greetingMessage"`
}

type UpdateRequest struct {
	Name            *string `json:"name,omitempty"`
	Color           *string `json:"color,omitempty"`
	GreetingMessage *string `json:"greetingMessage,omitempty"`
}
