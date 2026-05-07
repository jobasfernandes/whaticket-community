package user

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
)

type User struct {
	ID           uint          `gorm:"primaryKey;autoIncrement"`
	Name         string        `gorm:"size:255;not null"`
	Email        string        `gorm:"size:255;not null;uniqueIndex:users_email_uniq"`
	PasswordHash string        `gorm:"column:password_hash;size:255;not null" json:"-"`
	Password     string        `gorm:"-" json:"-"`
	Profile      string        `gorm:"size:50;not null;default:admin"`
	WhatsappID   *uint         `gorm:"column:whatsapp_id"`
	Whatsapp     *Whatsapp     `gorm:"foreignKey:WhatsappID"`
	Queues       []queue.Queue `gorm:"many2many:user_queues;"`
	TokenVersion int           `gorm:"column:token_version;not null;default:0" json:"-"`
	CreatedAt    time.Time     `gorm:"not null;default:now()"`
	UpdatedAt    time.Time     `gorm:"not null;default:now()"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.Password == "" {
		return nil
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hashed)
	u.Password = ""
	return nil
}

type Whatsapp struct {
	ID   uint   `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"size:255"`
}

func (Whatsapp) TableName() string {
	return "whatsapps"
}

type QueueDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type WhatsappDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type UserDTO struct {
	ID        uint         `json:"id"`
	Name      string       `json:"name"`
	Email     string       `json:"email"`
	Profile   string       `json:"profile"`
	CreatedAt time.Time    `json:"createdAt"`
	Queues    []QueueDTO   `json:"queues"`
	Whatsapp  *WhatsappDTO `json:"whatsapp"`
}

func Serialize(u *User) UserDTO {
	queues := make([]QueueDTO, 0, len(u.Queues))
	for i := range u.Queues {
		queues = append(queues, QueueDTO{
			ID:    u.Queues[i].ID,
			Name:  u.Queues[i].Name,
			Color: u.Queues[i].Color,
		})
	}
	var whatsapp *WhatsappDTO
	if u.Whatsapp != nil {
		whatsapp = &WhatsappDTO{ID: u.Whatsapp.ID, Name: u.Whatsapp.Name}
	}
	return UserDTO{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Profile:   u.Profile,
		CreatedAt: u.CreatedAt,
		Queues:    queues,
		Whatsapp:  whatsapp,
	}
}
