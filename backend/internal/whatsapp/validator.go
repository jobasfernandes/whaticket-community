package whatsapp

import (
	stdErrors "errors"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const (
	errInvalidName          = "ERR_INVALID_NAME"
	errInvalidInput         = "ERR_INVALID_INPUT"
	errInvalidMediaDelivery = "ERR_INVALID_MEDIA_DELIVERY"
	errGreetingRequired     = "ERR_WAPP_GREETING_REQUIRED"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type CreateRequest struct {
	Name             string            `json:"name" validate:"required,min=2,max=255"`
	QueueIDs         []uint            `json:"queueIds"`
	GreetingMessage  string            `json:"greetingMessage"`
	FarewellMessage  string            `json:"farewellMessage"`
	IsDefault        *bool             `json:"isDefault"`
	AdvancedSettings *AdvancedSettings `json:"advancedSettings"`
	MediaDelivery    string            `json:"-"`
}

type UpdateRequest struct {
	Name             *string           `json:"name" validate:"omitempty,min=2,max=255"`
	QueueIDs         *[]uint           `json:"queueIds"`
	GreetingMessage  *string           `json:"greetingMessage"`
	FarewellMessage  *string           `json:"farewellMessage"`
	IsDefault        *bool             `json:"isDefault"`
	AdvancedSettings *AdvancedSettings `json:"advancedSettings"`
	MediaDelivery    *string           `json:"-"`
}

func trimCreate(req *CreateRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.GreetingMessage = strings.TrimSpace(req.GreetingMessage)
	req.FarewellMessage = strings.TrimSpace(req.FarewellMessage)
	req.MediaDelivery = strings.TrimSpace(req.MediaDelivery)
}

func trimUpdate(req *UpdateRequest) {
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		req.Name = &v
	}
	if req.GreetingMessage != nil {
		v := strings.TrimSpace(*req.GreetingMessage)
		req.GreetingMessage = &v
	}
	if req.FarewellMessage != nil {
		v := strings.TrimSpace(*req.FarewellMessage)
		req.FarewellMessage = &v
	}
	if req.MediaDelivery != nil {
		v := strings.TrimSpace(*req.MediaDelivery)
		req.MediaDelivery = &v
	}
}

func validateCreate(req *CreateRequest) *errors.AppError {
	if err := validate.Struct(req); err != nil {
		return mapValidationError(err)
	}
	if len(req.QueueIDs) >= 2 && req.GreetingMessage == "" {
		return errors.New(errGreetingRequired, http.StatusBadRequest)
	}
	return nil
}

func validateUpdate(req *UpdateRequest) *errors.AppError {
	if err := validate.Struct(req); err != nil {
		return mapValidationError(err)
	}
	if req.QueueIDs != nil && len(*req.QueueIDs) >= 2 {
		if req.GreetingMessage != nil && *req.GreetingMessage == "" {
			return errors.New(errGreetingRequired, http.StatusBadRequest)
		}
	}
	return nil
}

func mapValidationError(err error) *errors.AppError {
	var ve validator.ValidationErrors
	if !stdErrors.As(err, &ve) {
		return errors.New(errInvalidInput, http.StatusBadRequest)
	}
	for _, fe := range ve {
		switch fe.Field() {
		case "Name":
			return errors.New(errInvalidName, http.StatusBadRequest)
		case "MediaDelivery":
			return errors.New(errInvalidMediaDelivery, http.StatusBadRequest)
		}
	}
	return errors.New(errInvalidInput, http.StatusBadRequest)
}
