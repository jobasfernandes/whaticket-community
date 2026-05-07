package queue

import (
	stdErrors "errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const (
	errInvalidName  = "ERR_QUEUE_INVALID_NAME"
	errInvalidColor = "ERR_QUEUE_INVALID_COLOR"
)

var colorHexRegex = regexp.MustCompile("(?i)^#[0-9a-f]{3,6}$")

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.RegisterValidation("colorHex", colorHexValidator); err != nil {
		panic(err)
	}
	return v
}

func colorHexValidator(fl validator.FieldLevel) bool {
	return colorHexRegex.MatchString(fl.Field().String())
}

type createRequestRules struct {
	Name  string `validate:"required,min=2,max=255"`
	Color string `validate:"required,colorHex"`
}

type updateRequestRules struct {
	Name  *string `validate:"omitempty,min=2,max=255"`
	Color *string `validate:"omitempty,colorHex"`
}

func trimCreate(req *CreateRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Color = strings.TrimSpace(req.Color)
}

func trimUpdate(req *UpdateRequest) {
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		req.Name = &v
	}
	if req.Color != nil {
		v := strings.TrimSpace(*req.Color)
		req.Color = &v
	}
}

func validateCreate(req *CreateRequest) *errors.AppError {
	err := validate.Struct(createRequestRules{Name: req.Name, Color: req.Color})
	if err == nil {
		return nil
	}
	return mapValidationError(err)
}

func validateUpdate(req *UpdateRequest) *errors.AppError {
	err := validate.Struct(updateRequestRules{Name: req.Name, Color: req.Color})
	if err == nil {
		return nil
	}
	return mapValidationError(err)
}

func mapValidationError(err error) *errors.AppError {
	var ve validator.ValidationErrors
	if !stdErrors.As(err, &ve) {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	for _, fe := range ve {
		switch fe.Field() {
		case "Name":
			return errors.New(errInvalidName, http.StatusBadRequest)
		case "Color":
			return errors.New(errInvalidColor, http.StatusBadRequest)
		}
	}
	return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
}
