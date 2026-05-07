package message

import (
	"encoding/json"
	stdErrors "errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

const (
	errPayloadTooLarge   = "ERR_PAYLOAD_TOO_LARGE"
	errBadRequest        = "ERR_BAD_REQUEST"
	errInvalidBody       = "ERR_INVALID_MESSAGE_BODY"
	mediaFormField       = "medias[]"
	bodyFormField        = "body"
	quotedMsgFormField   = "quotedMsg"
	maxBodyLengthSetting = 4096
)

type SendTextRequest struct {
	Body      string     `json:"body" validate:"required,max=4096"`
	QuotedMsg *QuotedRef `json:"quotedMsg,omitempty"`
}

type QuotedRef struct {
	ID string `json:"id" validate:"required"`
}

type FileUpload struct {
	Filename    string
	ContentType string
	Data        []byte
}

var sendValidator = validator.New(validator.WithRequiredStructEnabled())

func decodeSendRequest(r *http.Request, maxSize int64) (string, *QuotedRef, []FileUpload, *errors.AppError) {
	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		return decodeJSONSendRequest(r)
	case strings.HasPrefix(contentType, "multipart/"):
		return decodeMultipartSendRequest(r, maxSize)
	default:
		return "", nil, nil, errors.New(errBadRequest, http.StatusBadRequest)
	}
}

func decodeJSONSendRequest(r *http.Request) (string, *QuotedRef, []FileUpload, *errors.AppError) {
	var req SendTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", nil, nil, errors.New(errBadRequest, http.StatusBadRequest)
	}
	if err := sendValidator.Struct(&req); err != nil {
		return "", nil, nil, mapSendValidationError(err)
	}
	return req.Body, req.QuotedMsg, nil, nil
}

func decodeMultipartSendRequest(r *http.Request, maxSize int64) (string, *QuotedRef, []FileUpload, *errors.AppError) {
	if maxSize <= 0 {
		maxSize = 16 << 20
	}
	r.Body = http.MaxBytesReader(nil, r.Body, maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		var maxBytesErr *http.MaxBytesError
		if stdErrors.As(err, &maxBytesErr) {
			return "", nil, nil, errors.New(errPayloadTooLarge, http.StatusRequestEntityTooLarge)
		}
		return "", nil, nil, errors.New(errBadRequest, http.StatusBadRequest)
	}
	body := r.FormValue(bodyFormField)
	if len(body) > maxBodyLengthSetting {
		return "", nil, nil, errors.New(errInvalidBody, http.StatusBadRequest)
	}
	var quoted *QuotedRef
	if raw := r.FormValue(quotedMsgFormField); raw != "" {
		ref, qErr := parseQuotedRef(raw)
		if qErr != nil {
			return "", nil, nil, qErr
		}
		quoted = ref
	}
	files, fErr := readMultipartFiles(r.MultipartForm)
	if fErr != nil {
		return "", nil, nil, fErr
	}
	if body == "" && len(files) == 0 {
		return "", nil, nil, errors.New(errBadRequest, http.StatusBadRequest)
	}
	return body, quoted, files, nil
}

func parseQuotedRef(raw string) (*QuotedRef, *errors.AppError) {
	var ref QuotedRef
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		return nil, errors.New(errBadRequest, http.StatusBadRequest)
	}
	if err := sendValidator.Struct(&ref); err != nil {
		return nil, errors.New(errBadRequest, http.StatusBadRequest)
	}
	return &ref, nil
}

func readMultipartFiles(form *multipart.Form) ([]FileUpload, *errors.AppError) {
	if form == nil {
		return nil, nil
	}
	headers := form.File[mediaFormField]
	if len(headers) == 0 {
		return nil, nil
	}
	uploads := make([]FileUpload, 0, len(headers))
	for _, header := range headers {
		upload, err := readMultipartFile(header)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

func readMultipartFile(header *multipart.FileHeader) (FileUpload, *errors.AppError) {
	file, err := header.Open()
	if err != nil {
		return FileUpload{}, errors.Wrap(err, errBadRequest, http.StatusBadRequest)
	}
	defer file.Close()
	data, readErr := io.ReadAll(file)
	if readErr != nil {
		var maxBytesErr *http.MaxBytesError
		if stdErrors.As(readErr, &maxBytesErr) {
			return FileUpload{}, errors.New(errPayloadTooLarge, http.StatusRequestEntityTooLarge)
		}
		return FileUpload{}, errors.Wrap(readErr, errBadRequest, http.StatusBadRequest)
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(sniffPrefix(data))
	}
	return FileUpload{
		Filename:    header.Filename,
		ContentType: contentType,
		Data:        data,
	}, nil
}

func sniffPrefix(data []byte) []byte {
	if len(data) > 512 {
		return data[:512]
	}
	return data
}

func extToKind(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	switch {
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	case strings.HasPrefix(ct, "video/"):
		return "video"
	default:
		return "document"
	}
}

func mapSendValidationError(err error) *errors.AppError {
	var ve validator.ValidationErrors
	if !stdErrors.As(err, &ve) {
		return errors.New(errBadRequest, http.StatusBadRequest)
	}
	return errors.New(errInvalidBody, http.StatusBadRequest)
}
