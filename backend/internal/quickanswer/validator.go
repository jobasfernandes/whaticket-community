package quickanswer

import (
	"net/http"
	"strings"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const (
	shortcutMin = 1
	shortcutMax = 255
	messageMin  = 1
	messageMax  = 4000
)

func trim(req *CreateRequest) {
	req.Shortcut = strings.TrimSpace(req.Shortcut)
	req.Message = strings.TrimSpace(req.Message)
}

func trimUpdate(req *UpdateRequest) {
	if req.Shortcut != nil {
		v := strings.TrimSpace(*req.Shortcut)
		req.Shortcut = &v
	}
	if req.Message != nil {
		v := strings.TrimSpace(*req.Message)
		req.Message = &v
	}
}

func validateCreate(req *CreateRequest) *errors.AppError {
	if l := len(req.Shortcut); l < shortcutMin || l > shortcutMax {
		return errors.New("ERR_INVALID_SHORTCUT", http.StatusBadRequest)
	}
	if l := len(req.Message); l < messageMin || l > messageMax {
		return errors.New("ERR_INVALID_MESSAGE", http.StatusBadRequest)
	}
	return nil
}

func validateUpdate(req *UpdateRequest) *errors.AppError {
	if req.Shortcut != nil {
		if l := len(*req.Shortcut); l < shortcutMin || l > shortcutMax {
			return errors.New("ERR_INVALID_SHORTCUT", http.StatusBadRequest)
		}
	}
	if req.Message != nil {
		if l := len(*req.Message); l < messageMin || l > messageMax {
			return errors.New("ERR_INVALID_MESSAGE", http.StatusBadRequest)
		}
	}
	return nil
}
