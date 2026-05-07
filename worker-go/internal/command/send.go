package command

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"github.com/jobasfernandes/whaticket-go-worker/internal/linkpreview"
	"github.com/jobasfernandes/whaticket-go-worker/internal/media"
	apperrors "github.com/jobasfernandes/whaticket-go-worker/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-worker/internal/rmq"
	whatsmeowpkg "github.com/jobasfernandes/whaticket-go-worker/internal/whatsmeow"
)

const linkPreviewTimeout = 5 * time.Second

const maxOutboundMediaBytes = 16 << 20

const (
	defaultPTTMime   = "audio/ogg; codecs=opus"
	defaultAudioMime = "audio/mpeg"
	stickerMime      = "image/webp"
)

func (h *Handlers) requireLiveSession(connID int) (*whatsmeowpkg.Session, error) {
	sess, ok := h.Mgr.Get(connID)
	if !ok || sess == nil || sess.Client == nil {
		return nil, apperrors.New(apperrors.ErrNoSession, http.StatusNotFound)
	}
	if !sess.Client.IsLoggedIn() {
		return nil, apperrors.New(apperrors.ErrNotLoggedIn, http.StatusBadRequest)
	}
	return sess, nil
}

func (h *Handlers) sendMessage(ctx context.Context, sess *whatsmeowpkg.Session, recipient types.JID, msg *waE2E.Message, msgID types.MessageID) (SendResp, error) {
	resp, err := sess.Client.SendMessage(ctx, recipient, msg, whatsmeow.SendRequestExtra{ID: msgID})
	if err != nil {
		return SendResp{}, mapSendError(err)
	}
	return SendResp{ID: string(resp.ID), Timestamp: resp.Timestamp.Unix()}, nil
}

func mapSendError(err error) error {
	switch {
	case errors.Is(err, whatsmeow.ErrServerReturnedError):
		return apperrors.Wrap(err, apperrors.ErrServerRejected, http.StatusBadGateway)
	case errors.Is(err, whatsmeow.ErrNotConnected):
		return apperrors.Wrap(err, apperrors.ErrNoSession, http.StatusServiceUnavailable)
	case errors.Is(err, whatsmeow.ErrNotLoggedIn):
		return apperrors.Wrap(err, apperrors.ErrNotLoggedIn, http.StatusBadRequest)
	default:
		return apperrors.Wrap(err, apperrors.ErrInternal, http.StatusInternalServerError)
	}
}

func buildContextInfo(ci *ContextInfo) *waE2E.ContextInfo {
	if ci == nil || ci.StanzaID == "" {
		return nil
	}
	out := &waE2E.ContextInfo{
		StanzaID:      proto.String(ci.StanzaID),
		QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
	}
	if ci.Participant != "" {
		out.Participant = proto.String(ci.Participant)
	}
	return out
}

func (h *Handlers) handleSendText(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendTextReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	msgID := sess.Client.GenerateMessageID()
	extended := &waE2E.ExtendedTextMessage{
		Text: proto.String(req.Body),
	}
	attachLinkPreview(ctx, extended, req.Body)
	if ctxInfo := buildContextInfo(req.ContextInfo); ctxInfo != nil {
		extended.ContextInfo = ctxInfo
	}
	msg := &waE2E.Message{ExtendedTextMessage: extended}
	return h.sendMessage(ctx, sess, recipient, msg, msgID)
}

func attachLinkPreview(ctx context.Context, extended *waE2E.ExtendedTextMessage, body string) {
	preview, ok := linkpreview.Extract(ctx, body, linkPreviewTimeout)
	if !ok || preview == nil {
		return
	}
	extended.MatchedText = proto.String(preview.URL)
	extended.Title = proto.String(preview.Title)
	if preview.Description != "" {
		extended.Description = proto.String(preview.Description)
	}
	if len(preview.Thumbnail) > 0 {
		extended.JPEGThumbnail = preview.Thumbnail
	}
}

func (h *Handlers) handleSendImage(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendImageReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}

	data, detectedMime, err := media.DecodeDataURLOrFetch(ctx, req.Image, maxOutboundMediaBytes)
	if err != nil {
		return nil, err
	}
	mimeType := pickMime(req.MimeType, detectedMime, "image/jpeg")

	uploaded, err := media.UploadOutbound(ctx, sess.Client, data, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	msgID := sess.Client.GenerateMessageID()
	imgMsg := &waE2E.ImageMessage{
		Caption:       proto.String(req.Caption),
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
	}
	if ctxInfo := buildContextInfo(req.ContextInfo); ctxInfo != nil {
		imgMsg.ContextInfo = ctxInfo
	}
	return h.sendMessage(ctx, sess, recipient, &waE2E.Message{ImageMessage: imgMsg}, msgID)
}

func (h *Handlers) handleSendAudio(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendAudioReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	data, detectedMime, err := media.DecodeDataURLOrFetch(ctx, req.Audio, maxOutboundMediaBytes)
	if err != nil {
		return nil, err
	}

	ptt := true
	if req.PTT != nil {
		ptt = *req.PTT
	}
	mimeFallback := defaultAudioMime
	if ptt {
		mimeFallback = defaultPTTMime
	}
	mimeType := pickMime(req.MimeType, detectedMime, mimeFallback)

	uploaded, err := media.UploadOutbound(ctx, sess.Client, data, whatsmeow.MediaAudio)
	if err != nil {
		return nil, err
	}

	msgID := sess.Client.GenerateMessageID()
	audioMsg := &waE2E.AudioMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
		PTT:           proto.Bool(ptt),
	}
	if req.Seconds > 0 {
		audioMsg.Seconds = proto.Uint32(req.Seconds)
	}
	if ctxInfo := buildContextInfo(req.ContextInfo); ctxInfo != nil {
		audioMsg.ContextInfo = ctxInfo
	}
	return h.sendMessage(ctx, sess, recipient, &waE2E.Message{AudioMessage: audioMsg}, msgID)
}

func (h *Handlers) handleSendVideo(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendVideoReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	data, detectedMime, err := media.DecodeDataURLOrFetch(ctx, req.Video, maxOutboundMediaBytes)
	if err != nil {
		return nil, err
	}
	mimeType := pickMime(req.MimeType, detectedMime, "video/mp4")
	uploaded, err := media.UploadOutbound(ctx, sess.Client, data, whatsmeow.MediaVideo)
	if err != nil {
		return nil, err
	}

	msgID := sess.Client.GenerateMessageID()
	videoMsg := &waE2E.VideoMessage{
		Caption:       proto.String(req.Caption),
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
	}
	if ctxInfo := buildContextInfo(req.ContextInfo); ctxInfo != nil {
		videoMsg.ContextInfo = ctxInfo
	}
	return h.sendMessage(ctx, sess, recipient, &waE2E.Message{VideoMessage: videoMsg}, msgID)
}

func (h *Handlers) handleSendDocument(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendDocumentReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.FileName) == "" {
		return nil, apperrors.New(apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	data, detectedMime, err := media.DecodeDataURLOrFetch(ctx, req.Document, maxOutboundMediaBytes)
	if err != nil {
		return nil, err
	}
	mimeType := pickMime(req.MimeType, detectedMime, "application/octet-stream")
	uploaded, err := media.UploadOutbound(ctx, sess.Client, data, whatsmeow.MediaDocument)
	if err != nil {
		return nil, err
	}

	msgID := sess.Client.GenerateMessageID()
	fileName := req.FileName
	docMsg := &waE2E.DocumentMessage{
		URL:           proto.String(uploaded.URL),
		FileName:      &fileName,
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
		Caption:       proto.String(req.Caption),
	}
	if ctxInfo := buildContextInfo(req.ContextInfo); ctxInfo != nil {
		docMsg.ContextInfo = ctxInfo
	}
	return h.sendMessage(ctx, sess, recipient, &waE2E.Message{DocumentMessage: docMsg}, msgID)
}

func (h *Handlers) handleSendSticker(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendStickerReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	rawBytes, _, err := media.DecodeDataURLOrFetch(ctx, req.Sticker, maxOutboundMediaBytes)
	if err != nil {
		return nil, err
	}
	stickerBytes, _, animated, err := media.ConvertToSticker(rawBytes)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	uploaded, err := media.UploadOutbound(ctx, sess.Client, stickerBytes, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	msgID := sess.Client.GenerateMessageID()
	stickerMsg := &waE2E.StickerMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(stickerMime),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(stickerBytes))),
	}
	if animated {
		stickerMsg.IsAnimated = proto.Bool(true)
	}
	return h.sendMessage(ctx, sess, recipient, &waE2E.Message{StickerMessage: stickerMsg}, msgID)
}

func (h *Handlers) handleSendContact(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendContactReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Vcard) == "" {
		return nil, apperrors.New(apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	msgID := sess.Client.GenerateMessageID()
	name := req.Name
	vcard := req.Vcard
	msg := &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: &name,
		Vcard:       &vcard,
	}}
	return h.sendMessage(ctx, sess, recipient, msg, msgID)
}

func (h *Handlers) handleSendLocation(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendLocationReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	msgID := sess.Client.GenerateMessageID()
	lat := req.Latitude
	lng := req.Longitude
	loc := &waE2E.LocationMessage{
		DegreesLatitude:  &lat,
		DegreesLongitude: &lng,
	}
	if req.Name != "" {
		name := req.Name
		loc.Name = &name
	}
	return h.sendMessage(ctx, sess, recipient, &waE2E.Message{LocationMessage: loc}, msgID)
}

func pickMime(explicit, detected, fallback string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(detected); v != "" {
		return v
	}
	return fallback
}
