package quickanswer

import "time"

type QuickAnswer struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Shortcut  string    `gorm:"size:255;not null;uniqueIndex:quick_answers_shortcut_uniq"`
	Message   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (QuickAnswer) TableName() string {
	return "quick_answers"
}

type QuickAnswerDTO struct {
	ID        uint      `json:"id"`
	Shortcut  string    `json:"shortcut"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func Serialize(q *QuickAnswer) QuickAnswerDTO {
	return QuickAnswerDTO{
		ID:        q.ID,
		Shortcut:  q.Shortcut,
		Message:   q.Message,
		CreatedAt: q.CreatedAt,
		UpdatedAt: q.UpdatedAt,
	}
}

type CreateRequest struct {
	Shortcut string `json:"shortcut"`
	Message  string `json:"message"`
}

type UpdateRequest struct {
	Shortcut *string `json:"shortcut,omitempty"`
	Message  *string `json:"message,omitempty"`
}
