package contact

import "time"

type ContactCustomField struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	ContactID uint      `gorm:"column:contact_id;not null;index:ccf_contact_id_idx"`
	Name      string    `gorm:"size:255;not null"`
	Value     string    `gorm:"type:text;not null;default:''"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (ContactCustomField) TableName() string {
	return "contact_custom_fields"
}

type CustomFieldDTO struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func SerializeCustomField(f *ContactCustomField) CustomFieldDTO {
	return CustomFieldDTO{
		ID:        f.ID,
		Name:      f.Name,
		Value:     f.Value,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
}

type CustomFieldData struct {
	ID    *uint  `json:"id,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}
