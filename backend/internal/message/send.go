package message

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/contact"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
	"github.com/canove/whaticket-community/backend/internal/platform/httpx"
)

const (
	errMediaUnsupported = "ERR_MEDIA_NOT_SUPPORTED"
	errNoSender         = "ERR_SENDER_NOT_CONFIGURED"
	errMediaTooLarge    = "ERR_MEDIA_TOO_LARGE"
	errMediaParse       = "ERR_MEDIA_PARSE"
	errMediaEmpty       = "ERR_MEDIA_EMPTY"
)

const (
	multipartFieldMedias = "medias"
	multipartFieldBody   = "body"
	defaultMaxMediaBytes = 16 << 20
	mediaSniffBytes      = 512
)

const (
	kindImage    = "image"
	kindAudio    = "audio"
	kindVideo    = "video"
	kindDocument = "document"
	kindSticker  = "sticker"
)

type sendBody struct {
	Body      string         `json:"body"`
	FromMe    bool           `json:"fromMe"`
	Read      any            `json:"read"`
	MediaURL  string         `json:"mediaUrl"`
	QuotedMsg map[string]any `json:"quotedMsg"`
}

type sendMultiResponse struct {
	Messages []MessageDTO `json:"messages"`
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) error {
	ticketID, appErr := parseTicketID(r)
	if appErr != nil {
		return appErr
	}
	actor := claimsPointer(r)
	if actor == nil {
		return errors.New("ERR_SESSION_EXPIRED", http.StatusUnauthorized)
	}

	ticketLike, appErr := h.Deps.TicketSvc.Show(r.Context(), ticketID, actor)
	if appErr != nil {
		return appErr
	}
	contactNumber, appErr := h.Deps.lookupContactNumber(r.Context(), ticketLike.GetContactID())
	if appErr != nil {
		return appErr
	}
	if h.Deps.Sender == nil {
		return errors.New(errNoSender, http.StatusServiceUnavailable)
	}

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return h.sendMultipart(w, r, ticketID, ticketLike, contactNumber)
	}

	return h.sendJSON(w, r, ticketID, ticketLike, contactNumber)
}

func (h *Handler) sendJSON(w http.ResponseWriter, r *http.Request, ticketID uint, ticketLike TicketLike, contactNumber string) error {
	var req sendBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		return errors.New(errInvalidInput, http.StatusBadRequest)
	}

	quotedID := extractQuotedID(req.QuotedMsg)
	id, _, sendErr := h.Deps.Sender.SendText(r.Context(), ticketLike.GetWhatsappID(), contactNumber, body, quotedID)
	if sendErr != nil {
		return sendErr
	}

	contactID := ticketLike.GetContactID()
	data := MessageData{
		ID:        id,
		TicketID:  ticketID,
		ContactID: &contactID,
		Body:      body,
		MediaType: "chat",
		MediaURL:  req.MediaURL,
		FromMe:    true,
		Read:      true,
		Ack:       0,
	}
	if quotedID != "" {
		q := quotedID
		data.QuotedMsgID = &q
	}

	created, appErr := h.Deps.Create(r.Context(), data)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusCreated, Serialize(created))
	return nil
}

func (h *Handler) sendMultipart(w http.ResponseWriter, r *http.Request, ticketID uint, ticketLike TicketLike, contactNumber string) error {
	maxBytes := mediaMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		if isRequestTooLarge(err) {
			return errors.Wrap(err, errMediaTooLarge, http.StatusRequestEntityTooLarge)
		}
		return errors.Wrap(err, errMediaParse, http.StatusBadRequest)
	}
	form := r.MultipartForm
	if form == nil {
		return errors.New(errMediaParse, http.StatusBadRequest)
	}

	files := form.File[multipartFieldMedias]
	if len(files) == 0 {
		return errors.New(errMediaEmpty, http.StatusBadRequest)
	}

	caption := strings.TrimSpace(formFirstValue(form.Value, multipartFieldBody))
	whatsappID := ticketLike.GetWhatsappID()
	contactID := ticketLike.GetContactID()
	dtos := make([]MessageDTO, 0, len(files))

	for _, fh := range files {
		dto, appErr := h.dispatchMultipartFile(r.Context(), ticketID, whatsappID, contactID, contactNumber, caption, fh)
		if appErr != nil {
			return appErr
		}
		dtos = append(dtos, dto)
	}

	httpx.WriteJSON(w, http.StatusCreated, sendMultiResponse{Messages: dtos})
	return nil
}

func (h *Handler) dispatchMultipartFile(ctx context.Context, ticketID, whatsappID uint, contactID uint, to, caption string, fh *multipart.FileHeader) (MessageDTO, *errors.AppError) {
	data, mimeType, appErr := readMultipartFile(fh)
	if appErr != nil {
		return MessageDTO{}, appErr
	}
	dataURL := encodeDataURL(data, mimeType)
	kind := classifyMime(mimeType, fh.Filename)
	displayName := fh.Filename
	if displayName == "" {
		displayName = fmt.Sprintf("file-%s", filepath.Ext(mimeType))
	}
	body := caption
	if body == "" {
		body = displayName
	}

	id, sendErr := h.dispatchSend(ctx, kind, whatsappID, to, dataURL, mimeType, displayName, caption)
	if sendErr != nil {
		return MessageDTO{}, sendErr
	}

	mediaURL := h.uploadOutboundMedia(ctx, ticketID, id, displayName, data, mimeType)

	cid := contactID
	created, appErr := h.Deps.Create(ctx, MessageData{
		ID:        id,
		TicketID:  ticketID,
		ContactID: &cid,
		Body:      body,
		MediaType: kind,
		MediaURL:  mediaURL,
		FromMe:    true,
		Read:      true,
		Ack:       0,
	})
	if appErr != nil {
		return MessageDTO{}, appErr
	}
	return Serialize(created), nil
}

func (h *Handler) uploadOutboundMedia(ctx context.Context, ticketID uint, msgID, fileName string, data []byte, mimeType string) string {
	if h.Deps == nil || h.Deps.MediaUploader == nil {
		return ""
	}
	safeName := sanitizeObjectName(fileName)
	if safeName == "" {
		safeName = msgID
	}
	objectKey := fmt.Sprintf("wa-outbound/%d/%s/%s", ticketID, msgID, safeName)
	publicURL, err := h.Deps.MediaUploader.Upload(ctx, objectKey, data, mimeType)
	if err != nil {
		logger := h.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("outbound media upload failed",
			slog.String("object_key", objectKey),
			slog.Any("err", err),
		)
		return ""
	}
	return publicURL
}

func sanitizeObjectName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	trimmed = filepath.Base(trimmed)
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func (h *Handler) dispatchSend(ctx context.Context, kind string, whatsappID uint, to, dataURL, mimeType, fileName, caption string) (string, *errors.AppError) {
	switch kind {
	case kindImage:
		id, _, err := h.Deps.Sender.SendImage(ctx, whatsappID, to, dataURL, caption, mimeType, "")
		return id, err
	case kindAudio:
		id, _, err := h.Deps.Sender.SendAudio(ctx, whatsappID, to, dataURL, mimeType, true, "")
		return id, err
	case kindVideo:
		id, _, err := h.Deps.Sender.SendVideo(ctx, whatsappID, to, dataURL, caption, mimeType, "")
		return id, err
	case kindSticker:
		id, _, err := h.Deps.Sender.SendSticker(ctx, whatsappID, to, dataURL, "")
		return id, err
	default:
		id, _, err := h.Deps.Sender.SendDocument(ctx, whatsappID, to, dataURL, fileName, caption, mimeType, "")
		return id, err
	}
}

func readMultipartFile(fh *multipart.FileHeader) ([]byte, string, *errors.AppError) {
	f, err := fh.Open()
	if err != nil {
		return nil, "", errors.Wrap(err, errMediaParse, http.StatusBadRequest)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, "", errors.Wrap(err, errMediaParse, http.StatusBadRequest)
	}
	if len(data) == 0 {
		return nil, "", errors.New(errMediaEmpty, http.StatusBadRequest)
	}
	mimeType := strings.TrimSpace(fh.Header.Get("Content-Type"))
	if mimeType == "" || mimeType == "application/octet-stream" {
		head := data
		if len(head) > mediaSniffBytes {
			head = head[:mediaSniffBytes]
		}
		mimeType = http.DetectContentType(head)
	}
	return data, mimeType, nil
}

func encodeDataURL(data []byte, mimeType string) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	mt := mimeType
	if mt == "" {
		mt = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", mt, encoded)
}

func classifyMime(mimeType, fileName string) string {
	mt := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case mt == "image/webp" && fileName != "" && strings.EqualFold(filepath.Ext(fileName), ".webp"):
		return kindImage
	case strings.HasPrefix(mt, "image/"):
		return kindImage
	case strings.HasPrefix(mt, "audio/"):
		return kindAudio
	case strings.HasPrefix(mt, "video/"):
		return kindVideo
	default:
		return kindDocument
	}
}

func formFirstValue(values map[string][]string, key string) string {
	if values == nil {
		return ""
	}
	if list, ok := values[key]; ok && len(list) > 0 {
		return list[0]
	}
	return ""
}

func isRequestTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "http: request body too large") || strings.Contains(msg, "multipart: message too large")
}

func mediaMaxBytes() int64 {
	if raw := strings.TrimSpace(os.Getenv("MEDIA_MAX_SIZE")); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			return v
		}
	}
	return defaultMaxMediaBytes
}

func (d *Deps) lookupContactNumber(ctx context.Context, contactID uint) (string, *errors.AppError) {
	if contactID == 0 {
		return "", errors.New(errInvalidInput, http.StatusBadRequest)
	}
	var c contact.Contact
	err := d.DB.WithContext(ctx).Select("number", "is_group").Where("id = ?", contactID).Take(&c).Error
	if err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("ERR_NO_CONTACT_FOUND", http.StatusNotFound)
		}
		return "", errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}
	number := c.Number
	if number == "" {
		return "", errors.New("ERR_NO_CONTACT_NUMBER", http.StatusBadRequest)
	}
	return number, nil
}

func extractQuotedID(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["id"].(string); ok {
		return v
	}
	return ""
}
