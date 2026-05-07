package user

import (
	stdErrors "errors"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const (
	errInvalidName     = "ERR_INVALID_NAME"
	errInvalidEmail    = "ERR_INVALID_EMAIL"
	errInvalidPassword = "ERR_INVALID_PASSWORD"
	errInvalidProfile  = "ERR_INVALID_PROFILE"
	errInvalidQueue    = "ERR_INVALID_QUEUE"
	errInvalidInput    = "ERR_INVALID_INPUT"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type CreateRequest struct {
	Name       string `json:"name" validate:"required,min=2,max=255"`
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required,min=5"`
	Profile    string `json:"profile" validate:"omitempty,oneof=admin user"`
	QueueIDs   []uint `json:"queueIds" validate:"omitempty,dive,gt=0"`
	WhatsappID *uint  `json:"whatsappId" validate:"omitempty,gt=0"`
}

type UpdateRequest struct {
	Name       *string `json:"name,omitempty"`
	Email      *string `json:"email,omitempty"`
	Password   *string `json:"password,omitempty"`
	Profile    *string `json:"profile,omitempty" validate:"omitempty,oneof=admin user"`
	QueueIDs   *[]uint `json:"queueIds,omitempty"`
	WhatsappID **uint  `json:"whatsappId,omitempty"`
}

func validateCreate(req *CreateRequest) *errors.AppError {
	if err := validate.Struct(req); err != nil {
		return mapFieldError(err)
	}
	return nil
}

func validateUpdate(req *UpdateRequest) *errors.AppError {
	if req.Name != nil {
		if l := len(*req.Name); l < 2 || l > 255 {
			return errors.New(errInvalidName, http.StatusBadRequest)
		}
	}
	if req.Email != nil {
		if err := validate.Var(*req.Email, "required,email"); err != nil {
			return errors.New(errInvalidEmail, http.StatusBadRequest)
		}
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 5 {
			return errors.New(errInvalidPassword, http.StatusBadRequest)
		}
	}
	if req.Profile != nil {
		if *req.Profile != "admin" && *req.Profile != "user" {
			return errors.New(errInvalidProfile, http.StatusBadRequest)
		}
	}
	if req.QueueIDs != nil {
		for _, id := range *req.QueueIDs {
			if id == 0 {
				return errors.New(errInvalidQueue, http.StatusBadRequest)
			}
		}
	}
	return nil
}

func mapFieldError(err error) *errors.AppError {
	var ve validator.ValidationErrors
	if !stdErrors.As(err, &ve) {
		return errors.New(errInvalidInput, http.StatusBadRequest)
	}
	for _, fe := range ve {
		switch fe.Field() {
		case "Name":
			return errors.New(errInvalidName, http.StatusBadRequest)
		case "Email":
			return errors.New(errInvalidEmail, http.StatusBadRequest)
		case "Password":
			return errors.New(errInvalidPassword, http.StatusBadRequest)
		case "Profile":
			return errors.New(errInvalidProfile, http.StatusBadRequest)
		case "QueueIDs":
			return errors.New(errInvalidQueue, http.StatusBadRequest)
		}
	}
	return errors.New(errInvalidInput, http.StatusBadRequest)
}
