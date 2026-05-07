package httpx

import (
	"encoding/json"
	stdErrors "errors"
	"log/slog"
	"net/http"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func Wrap(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			writeError(w, err)
		}
	}
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *errors.AppError
	if stdErrors.As(err, &appErr) {
		WriteJSON(w, appErr.Status, map[string]string{"error": appErr.Code})
		return
	}
	slog.Error("unhandled error in handler", "err", err)
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, err error) {
	writeError(w, err)
}
