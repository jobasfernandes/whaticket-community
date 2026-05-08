package contact

import (
	"strings"
	"time"
)

type Contact struct {
	ID            uint                 `gorm:"primaryKey;autoIncrement"`
	Name          string               `gorm:"size:255;not null;default:''"`
	Number        string               `gorm:"size:255;not null;default:''"`
	LID           string               `gorm:"column:lid;size:255;not null;default:''"`
	Email         string               `gorm:"size:255;not null;default:''"`
	ProfilePicURL string               `gorm:"column:profile_pic_url;type:text;not null;default:''"`
	IsGroup       bool                 `gorm:"column:is_group;not null;default:false"`
	ExtraInfo     []ContactCustomField `gorm:"foreignKey:ContactID"`
	CreatedAt     time.Time            `gorm:"not null;default:now()"`
	UpdatedAt     time.Time            `gorm:"not null;default:now()"`
}

func (Contact) TableName() string {
	return "contacts"
}

func (c *Contact) JID() string {
	if strings.Contains(c.Number, "@") {
		return c.Number
	}
	if c.IsGroup {
		return c.Number + "@g.us"
	}
	return c.Number + "@s.whatsapp.net"
}

type ContactDTO struct {
	ID            uint             `json:"id"`
	Name          string           `json:"name"`
	Number        string           `json:"number"`
	LID           string           `json:"lid"`
	Email         string           `json:"email"`
	ProfilePicURL string           `json:"profilePicUrl"`
	IsGroup       bool             `json:"isGroup"`
	ExtraInfo     []CustomFieldDTO `json:"extraInfo"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
}

func Serialize(c *Contact) ContactDTO {
	extra := make([]CustomFieldDTO, 0, len(c.ExtraInfo))
	for i := range c.ExtraInfo {
		extra = append(extra, SerializeCustomField(&c.ExtraInfo[i]))
	}
	return ContactDTO{
		ID:            c.ID,
		Name:          c.Name,
		Number:        c.Number,
		LID:           c.LID,
		Email:         c.Email,
		ProfilePicURL: c.ProfilePicURL,
		IsGroup:       c.IsGroup,
		ExtraInfo:     extra,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

type CreateOrUpdateRequest struct {
	Number        string
	Name          string
	LID           string
	ProfilePicURL string
	IsGroup       bool
	Email         string
	ExtraInfo     []CustomFieldData
}

type CreateRequest struct {
	Name      string            `json:"name"`
	Number    string            `json:"number"`
	Email     string            `json:"email"`
	ExtraInfo []CustomFieldData `json:"extraInfo"`
}

type UpdateRequest struct {
	Name      *string            `json:"name,omitempty"`
	Number    *string            `json:"number,omitempty"`
	Email     *string            `json:"email,omitempty"`
	ExtraInfo *[]CustomFieldData `json:"extraInfo,omitempty"`
}
