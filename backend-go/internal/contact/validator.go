package contact

import (
	stdErrors "errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const (
	errInvalidName      = "ERR_INVALID_NAME"
	errInvalidNumber    = "ERR_INVALID_NUMBER"
	errInvalidEmail     = "ERR_INVALID_EMAIL"
	errInvalidExtraInfo = "ERR_INVALID_EXTRAINFO"
)

var (
	digitsOnlyRegex = regexp.MustCompile(`^\d{8,15}$`)
	validate        = validator.New(validator.WithRequiredStructEnabled())
)

type createRequestRules struct {
	Name  string `validate:"required,min=2,max=255"`
	Email string `validate:"omitempty,email"`
}

type updateRequestRules struct {
	Name  *string `validate:"omitempty,min=2,max=255"`
	Email *string `validate:"omitempty,email"`
}

func trimCreate(req *CreateRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Number = strings.TrimSpace(req.Number)
	req.Email = strings.TrimSpace(req.Email)
}

func trimUpdate(req *UpdateRequest) {
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		req.Name = &v
	}
	if req.Number != nil {
		v := strings.TrimSpace(*req.Number)
		req.Number = &v
	}
	if req.Email != nil {
		v := strings.TrimSpace(*req.Email)
		req.Email = &v
	}
}

func validateCreate(req *CreateRequest) *errors.AppError {
	if err := validate.Struct(createRequestRules{Name: req.Name, Email: req.Email}); err != nil {
		return mapValidationError(err)
	}
	if appErr := validateNumberFormat(req.Number); appErr != nil {
		return appErr
	}
	if appErr := validateExtraInfo(req.ExtraInfo); appErr != nil {
		return appErr
	}
	return nil
}

func validateUpdate(req *UpdateRequest) *errors.AppError {
	if err := validate.Struct(updateRequestRules{Name: req.Name, Email: req.Email}); err != nil {
		return mapValidationError(err)
	}
	if req.Number != nil {
		if appErr := validateNumberFormat(*req.Number); appErr != nil {
			return appErr
		}
	}
	if req.ExtraInfo != nil {
		if appErr := validateExtraInfo(*req.ExtraInfo); appErr != nil {
			return appErr
		}
	}
	return nil
}

func validateNumberFormat(raw string) *errors.AppError {
	normalized := NormalizeNumber(raw, false)
	if normalized == "" {
		return errors.New(errInvalidNumber, http.StatusBadRequest)
	}
	if !digitsOnlyRegex.MatchString(normalized) {
		return errors.New(errInvalidNumber, http.StatusBadRequest)
	}
	return nil
}

func validateExtraInfo(items []CustomFieldData) *errors.AppError {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(items))
	for i := range items {
		item := items[i]
		if item.ID != nil {
			if _, ok := seen[*item.ID]; ok {
				return errors.New(errInvalidExtraInfo, http.StatusBadRequest)
			}
			seen[*item.ID] = struct{}{}
		}
		if strings.TrimSpace(item.Name) == "" {
			return errors.New(errInvalidExtraInfo, http.StatusBadRequest)
		}
	}
	return nil
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
		case "Email":
			return errors.New(errInvalidEmail, http.StatusBadRequest)
		}
	}
	return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
}
