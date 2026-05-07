package waevents

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/mustache"
)

type MessagePayload struct {
	Event           MessageEvent `json:"event"`
	MimeType        string       `json:"mimeType"`
	FileName        string       `json:"fileName"`
	MediaURL        string       `json:"mediaUrl"`
	S3Key           string       `json:"s3Key"`
	Base64          string       `json:"base64"`
	IsSticker       bool         `json:"isSticker"`
	StickerAnimated bool         `json:"stickerAnimated"`
}

type MessageEvent struct {
	Info    MessageInfo    `json:"Info"`
	Message map[string]any `json:"Message"`
}

type MessageInfo struct {
	ID             string `json:"ID"`
	Sender         string `json:"Sender"`
	Chat           string `json:"Chat"`
	PushName       string `json:"PushName"`
	IsFromMe       bool   `json:"IsFromMe"`
	IsGroup        bool   `json:"IsGroup"`
	Type           string `json:"Type"`
	SenderAlt      string `json:"SenderAlt"`
	RecipientAlt   string `json:"RecipientAlt"`
	AddressingMode string `json:"AddressingMode"`
}

const (
	jidSuffixUser       = "@s.whatsapp.net"
	jidSuffixLID        = "@lid"
	jidSuffixGroup      = "@g.us"
	jidSuffixBroadcast  = "@broadcast"
	jidSuffixNewsletter = "@newsletter"
	jidStatusBroadcast  = "status@broadcast"
)

const (
	messageMediaTypeChat      = "chat"
	messageMediaTypeImage     = "image"
	messageMediaTypeAudio     = "audio"
	messageMediaTypeVideo     = "video"
	messageMediaTypeDocument  = "document"
	messageMediaTypeSticker   = "sticker"
	messageEmptyPlaceholder   = "(no content)"
	lrmRune                   = '‎'
	maxMessageBodyForFarewell = 4096
)

func (c *Consumer) handleMessage(ctx context.Context, whatsappID uint, payloadRaw []byte) error {
	var payload MessagePayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		c.Log.Warn("waevents message decode failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return nil
	}

	info := payload.Event.Info
	if info.ID == "" {
		c.Log.Error("waevents message with empty id, dropping",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
		)
		return nil
	}

	if shouldDropChat(info) {
		c.Log.Debug("waevents dropping non-1x1 chat",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("chat", info.Chat),
			slog.String("message_id", info.ID),
		)
		return nil
	}

	body := extractBody(payload.Event.Message)
	if startsWithLRM(body) {
		c.Log.Debug("waevents dropping lrm-prefixed message",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("message_id", info.ID),
		)
		return nil
	}

	if info.Sender == "" {
		c.Log.Error("waevents message with empty sender",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("message_id", info.ID),
		)
		return nil
	}

	contactJID, lidValue := resolveContactJID(info)
	if contactJID == "" {
		c.Log.Error("waevents could not resolve contact jid, dropping",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("message_id", info.ID),
		)
		return nil
	}
	contactNumber := jidToNumber(contactJID)
	contactName := info.PushName
	if info.IsFromMe {
		contactName = ""
	}
	contact, err := c.ContactSvc.CreateOrUpdate(ctx, contactNumber, contactName, lidValue, false)
	if err != nil {
		c.Log.Warn("waevents contact upsert failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return err
	}

	var groupContact Contact

	whatsapp, err := c.WhatsappSvc.Show(ctx, whatsappID)
	if err != nil {
		c.Log.Warn("waevents whatsapp show failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return err
	}

	if shouldSkipFarewellEcho(whatsapp, contact, info, body) {
		c.Log.Debug("waevents farewell echo dropped",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("message_id", info.ID),
		)
		return nil
	}

	unreadMessages := 0
	if !info.IsFromMe {
		unreadMessages = 1
	}

	ticket, err := c.TicketSvc.FindOrCreate(ctx, contact, whatsappID, unreadMessages, groupContact)
	if err != nil {
		c.Log.Warn("waevents ticket find or create failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return err
	}

	mediaURL := payload.MediaURL
	if mediaURL == "" && payload.Base64 != "" {
		c.Log.Warn("waevents base64-only media not supported in v1",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("message_id", info.ID),
			slog.String("mime_type", payload.MimeType),
		)
	}

	mediaType := deriveMediaType(payload, info, mediaURL)

	var contactID *uint
	if !info.IsFromMe {
		id := contact.GetID()
		contactID = &id
	}

	data := MessageData{
		ID:        info.ID,
		TicketID:  ticket.GetID(),
		ContactID: contactID,
		Body:      body,
		FromMe:    info.IsFromMe,
		Read:      info.IsFromMe,
		MediaType: mediaType,
		MediaURL:  mediaURL,
		Ack:       0,
	}

	lastMessageText := computeLastMessage(body, payload.FileName, mediaURL)
	if uerr := c.TicketSvc.UpdateLastMessage(ctx, ticket.GetID(), lastMessageText); uerr != nil {
		c.Log.Warn("waevents update last message failed",
			slog.Uint64("ticket_id", uint64(ticket.GetID())),
			slog.Any("err", uerr),
		)
	}

	if cerr := c.MessageSvc.Create(ctx, data); cerr != nil {
		c.Log.Warn("waevents message create failed",
			slog.Uint64("ticket_id", uint64(ticket.GetID())),
			slog.String("message_id", info.ID),
			slog.Any("err", cerr),
		)
		return cerr
	}

	if verr := c.processVcardMessage(ctx, info, body); verr != nil {
		c.Log.Warn("waevents vcard processing failed",
			slog.String("message_id", info.ID),
			slog.Any("err", verr),
		)
	}

	if shouldRunQueueLogic(whatsapp, ticket, groupContact, info.IsFromMe) {
		if qerr := c.handleQueueLogic(ctx, whatsapp, ticket, contact, body); qerr != nil {
			c.Log.Warn("waevents queue logic failed",
				slog.Uint64("ticket_id", uint64(ticket.GetID())),
				slog.Any("err", qerr),
			)
		}
	}

	return nil
}

func (c *Consumer) processVcardMessage(_ context.Context, _ MessageInfo, _ string) error {
	return nil
}

func extractBody(msg map[string]any) string {
	if msg == nil {
		return ""
	}
	if conv, ok := msg["conversation"].(string); ok && conv != "" {
		return conv
	}
	if ext, ok := msg["extendedTextMessage"].(map[string]any); ok {
		if text, ok := ext["text"].(string); ok && text != "" {
			return text
		}
	}
	for _, key := range []string{"imageMessage", "videoMessage", "documentMessage", "audioMessage"} {
		if media, ok := msg[key].(map[string]any); ok {
			if caption, ok := media["caption"].(string); ok && caption != "" {
				return caption
			}
		}
	}
	return ""
}

func startsWithLRM(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		return r == lrmRune
	}
	return false
}

func shouldDropChat(info MessageInfo) bool {
	chat := info.Chat
	if info.IsGroup {
		return true
	}
	if chat == "" {
		return true
	}
	if chat == jidStatusBroadcast {
		return true
	}
	if strings.HasSuffix(chat, jidSuffixGroup) {
		return true
	}
	if strings.HasSuffix(chat, jidSuffixBroadcast) {
		return true
	}
	if strings.HasSuffix(chat, jidSuffixNewsletter) {
		return true
	}
	if strings.HasSuffix(chat, jidSuffixUser) || strings.HasSuffix(chat, jidSuffixLID) {
		return false
	}
	return true
}

func resolveContactJID(info MessageInfo) (jid string, lid string) {
	primary := info.Chat
	if !info.IsFromMe && info.Sender != "" {
		primary = info.Sender
	}
	if info.IsFromMe && info.Chat != "" {
		primary = info.Chat
	}
	alt := ""
	if info.IsFromMe {
		alt = info.RecipientAlt
	} else {
		alt = info.SenderAlt
	}
	if strings.HasSuffix(primary, jidSuffixLID) {
		if strings.HasSuffix(alt, jidSuffixUser) {
			return alt, primary
		}
		return primary, primary
	}
	if strings.HasSuffix(primary, jidSuffixUser) {
		if strings.HasSuffix(alt, jidSuffixLID) {
			return primary, alt
		}
		return primary, ""
	}
	if strings.HasSuffix(alt, jidSuffixUser) {
		return alt, primary
	}
	return primary, ""
}

func jidToNumber(jid string) string {
	at := strings.IndexByte(jid, '@')
	if at <= 0 {
		return jid
	}
	number := jid[:at]
	if colon := strings.IndexByte(number, ':'); colon > 0 {
		number = number[:colon]
	}
	return number
}

func shouldSkipFarewellEcho(w Whatsapp, contact Contact, info MessageInfo, body string) bool {
	farewell := w.GetFarewellMessage()
	if farewell == "" || body == "" || info.IsFromMe {
		return false
	}
	if len(body) > maxMessageBodyForFarewell {
		return false
	}
	rendered := mustache.RenderOrTemplate(farewell, map[string]any{
		"name": contact.GetName(),
	})
	return rendered == body
}

func deriveMediaType(payload MessagePayload, info MessageInfo, mediaURL string) string {
	if payload.IsSticker {
		return messageMediaTypeSticker
	}
	hasMedia := mediaURL != "" || payload.Base64 != "" || payload.MimeType != ""
	if hasMedia {
		mime := strings.ToLower(strings.TrimSpace(payload.MimeType))
		switch {
		case strings.HasPrefix(mime, "image/"):
			return messageMediaTypeImage
		case strings.HasPrefix(mime, "audio/"):
			return messageMediaTypeAudio
		case strings.HasPrefix(mime, "video/"):
			return messageMediaTypeVideo
		case mime != "":
			return messageMediaTypeDocument
		}
	}
	if info.Type != "" {
		return info.Type
	}
	return messageMediaTypeChat
}

func computeLastMessage(body, fileName, mediaURL string) string {
	if body != "" {
		return body
	}
	if fileName != "" {
		return fileName
	}
	if mediaURL != "" {
		return mediaURL
	}
	return messageEmptyPlaceholder
}

func shouldRunQueueLogic(w Whatsapp, t Ticket, groupContact Contact, fromMe bool) bool {
	if fromMe {
		return false
	}
	if groupContact != nil {
		return false
	}
	if t.GetQueueID() != nil {
		return false
	}
	if t.GetUserID() != nil {
		return false
	}
	return len(w.GetQueues()) >= 1
}
