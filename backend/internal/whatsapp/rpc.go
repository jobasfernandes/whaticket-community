package whatsapp

import (
	"context"
	stdErrors "errors"
	"net/http"
	"time"

	"github.com/canove/whaticket-community/backend/internal/platform/errors"
	"github.com/canove/whaticket-community/backend/internal/rmq"
)

const (
	exchangeWaRPC              = "wa.rpc"
	routingMessageSendText     = "message.send.text"
	routingMessageSendImage    = "message.send.image"
	routingMessageSendAudio    = "message.send.audio"
	routingMessageSendVideo    = "message.send.video"
	routingMessageSendDocument = "message.send.document"
	routingMessageSendSticker  = "message.send.sticker"
	routingMessageSendContact  = "message.send.contact"
	routingMessageSendLocation = "message.send.location"
	routingMessageDelete       = "message.delete"
	routingMessageEdit         = "message.edit"
	routingMessageReact        = "message.react"
	routingMessageMarkRead     = "message.markread"
	routingContactCheckNumber  = "contact.checkNumber"
	routingContactProfilePic   = "contact.getProfilePic"
	routingContactInfo         = "contact.getInfo"
	routingPresenceSend        = "presence.send"
	routingChatPresenceSend    = "chat.presence.send"
	routingSessionPairPhone    = "session.pairphone"

	errWorkerUnavailable = "ERR_WORKER_UNAVAILABLE"
)

type RPCClient interface {
	Call(ctx context.Context, exchange, routingKey string, req any, resp any) error
}

type ContextInfo struct {
	StanzaID    string `json:"stanzaId,omitempty"`
	Participant string `json:"participant,omitempty"`
}

type messageSendResponse struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

type sendTextRequest struct {
	WhatsappID  uint         `json:"whatsappId"`
	To          string       `json:"to"`
	Body        string       `json:"body"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type sendImageRequest struct {
	WhatsappID  uint         `json:"whatsappId"`
	To          string       `json:"to"`
	Image       string       `json:"image"`
	Caption     string       `json:"caption,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type sendAudioRequest struct {
	WhatsappID  uint         `json:"whatsappId"`
	To          string       `json:"to"`
	Audio       string       `json:"audio"`
	PTT         *bool        `json:"ptt,omitempty"`
	Seconds     uint32       `json:"seconds,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type sendVideoRequest struct {
	WhatsappID    uint         `json:"whatsappId"`
	To            string       `json:"to"`
	Video         string       `json:"video"`
	Caption       string       `json:"caption,omitempty"`
	JpegThumbnail string       `json:"jpegThumbnail,omitempty"`
	MimeType      string       `json:"mimeType,omitempty"`
	ContextInfo   *ContextInfo `json:"contextInfo,omitempty"`
}

type sendDocumentRequest struct {
	WhatsappID  uint         `json:"whatsappId"`
	To          string       `json:"to"`
	Document    string       `json:"document"`
	FileName    string       `json:"fileName"`
	Caption     string       `json:"caption,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type sendStickerRequest struct {
	WhatsappID    uint         `json:"whatsappId"`
	To            string       `json:"to"`
	Sticker       string       `json:"sticker"`
	PackID        string       `json:"packId,omitempty"`
	PackName      string       `json:"packName,omitempty"`
	PackPublisher string       `json:"packPublisher,omitempty"`
	Emojis        []string     `json:"emojis,omitempty"`
	ContextInfo   *ContextInfo `json:"contextInfo,omitempty"`
}

type sendContactRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	To         string `json:"to"`
	Name       string `json:"name"`
	Vcard      string `json:"vcard"`
}

type sendLocationRequest struct {
	WhatsappID uint    `json:"whatsappId"`
	To         string  `json:"to"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Name       string  `json:"name,omitempty"`
}

type deleteMessageRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	To         string `json:"to"`
	ID         string `json:"id"`
}

type editMessageRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	To         string `json:"to"`
	ID         string `json:"id"`
	Body       string `json:"body"`
}

type reactMessageRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	To         string `json:"to"`
	ID         string `json:"id"`
	Emoji      string `json:"emoji"`
}

type markReadRequest struct {
	WhatsappID uint     `json:"whatsappId"`
	ChatJID    string   `json:"chatJID"`
	SenderJID  string   `json:"senderJID"`
	MessageIDs []string `json:"messageIds"`
	Timestamp  int64    `json:"timestamp"`
}

type checkNumberRequest struct {
	WhatsappID uint     `json:"whatsappId"`
	Numbers    []string `json:"numbers"`
}

type CheckResult struct {
	Number string `json:"number"`
	Exists bool   `json:"exists"`
	JID    string `json:"jid"`
}

type checkNumberResponse struct {
	Results []CheckResult `json:"results"`
}

type profilePicRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	JID        string `json:"jid"`
	Preview    bool   `json:"preview,omitempty"`
}

type profilePicResponse struct {
	URL string `json:"url"`
}

type contactInfoRequest struct {
	WhatsappID uint     `json:"whatsappId"`
	JIDs       []string `json:"jids"`
}

type ContactInfo struct {
	JID          string `json:"jid"`
	PushName     string `json:"pushName"`
	BusinessName string `json:"businessName"`
	Status       string `json:"status"`
}

type contactInfoResponse struct {
	Infos []ContactInfo `json:"infos"`
}

type presenceRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	Presence   string `json:"presence"`
}

type chatPresenceRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	ChatJID    string `json:"chatJID"`
	State      string `json:"state"`
}

type pairPhoneRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	Phone      string `json:"phone"`
}

type pairPhoneResponse struct {
	LinkingCode string `json:"linkingCode"`
}

func translateRPCError(err error) *errors.AppError {
	if err == nil {
		return nil
	}
	if stdErrors.Is(err, rmq.ErrNotConnected) || stdErrors.Is(err, rmq.ErrDisabled) || stdErrors.Is(err, rmq.ErrShuttingDown) || stdErrors.Is(err, rmq.ErrConnectionLost) {
		return errors.Wrap(err, errWorkerUnavailable, http.StatusServiceUnavailable)
	}
	var appErr *errors.AppError
	if stdErrors.As(err, &appErr) {
		return appErr
	}
	return errors.Wrap(err, errWorkerUnavailable, http.StatusServiceUnavailable)
}

func SendText(ctx context.Context, rpc RPCClient, whatsappID uint, to, body string, contextInfo *ContextInfo) (string, time.Time, *errors.AppError) {
	req := sendTextRequest{WhatsappID: whatsappID, To: to, Body: body, ContextInfo: contextInfo}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendText, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendImage(ctx context.Context, rpc RPCClient, whatsappID uint, to, image, caption, mimeType string, contextInfo *ContextInfo) (string, time.Time, *errors.AppError) {
	req := sendImageRequest{WhatsappID: whatsappID, To: to, Image: image, Caption: caption, MimeType: mimeType, ContextInfo: contextInfo}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendImage, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendAudio(ctx context.Context, rpc RPCClient, whatsappID uint, to, audio string, ptt bool, seconds uint32, contextInfo *ContextInfo, mimeType string) (string, time.Time, *errors.AppError) {
	pttPtr := ptt
	req := sendAudioRequest{
		WhatsappID:  whatsappID,
		To:          to,
		Audio:       audio,
		PTT:         &pttPtr,
		Seconds:     seconds,
		MimeType:    mimeType,
		ContextInfo: contextInfo,
	}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendAudio, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendVideo(ctx context.Context, rpc RPCClient, whatsappID uint, to, video, caption, jpegThumbnail string, contextInfo *ContextInfo, mimeType string) (string, time.Time, *errors.AppError) {
	req := sendVideoRequest{
		WhatsappID:    whatsappID,
		To:            to,
		Video:         video,
		Caption:       caption,
		JpegThumbnail: jpegThumbnail,
		MimeType:      mimeType,
		ContextInfo:   contextInfo,
	}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendVideo, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendDocument(ctx context.Context, rpc RPCClient, whatsappID uint, to, document, fileName, caption, mimeType string, contextInfo *ContextInfo) (string, time.Time, *errors.AppError) {
	req := sendDocumentRequest{WhatsappID: whatsappID, To: to, Document: document, FileName: fileName, Caption: caption, MimeType: mimeType, ContextInfo: contextInfo}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendDocument, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendSticker(ctx context.Context, rpc RPCClient, whatsappID uint, to, sticker, packID, packName, packPublisher string, emojis []string, contextInfo *ContextInfo) (string, time.Time, *errors.AppError) {
	req := sendStickerRequest{WhatsappID: whatsappID, To: to, Sticker: sticker, PackID: packID, PackName: packName, PackPublisher: packPublisher, Emojis: emojis, ContextInfo: contextInfo}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendSticker, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendContact(ctx context.Context, rpc RPCClient, whatsappID uint, to, name, vcard string) (string, time.Time, *errors.AppError) {
	req := sendContactRequest{WhatsappID: whatsappID, To: to, Name: name, Vcard: vcard}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendContact, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func SendLocation(ctx context.Context, rpc RPCClient, whatsappID uint, to string, latitude, longitude float64, name string) (string, time.Time, *errors.AppError) {
	req := sendLocationRequest{WhatsappID: whatsappID, To: to, Latitude: latitude, Longitude: longitude, Name: name}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageSendLocation, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func DeleteMessage(ctx context.Context, rpc RPCClient, whatsappID uint, to, msgID string) *errors.AppError {
	req := deleteMessageRequest{WhatsappID: whatsappID, To: to, ID: msgID}
	var resp struct{}
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageDelete, req, &resp); err != nil {
		return translateRPCError(err)
	}
	return nil
}

func EditMessage(ctx context.Context, rpc RPCClient, whatsappID uint, to, msgID, body string) (string, time.Time, *errors.AppError) {
	req := editMessageRequest{WhatsappID: whatsappID, To: to, ID: msgID, Body: body}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageEdit, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func ReactMessage(ctx context.Context, rpc RPCClient, whatsappID uint, to, msgID, emoji string) (string, time.Time, *errors.AppError) {
	req := reactMessageRequest{WhatsappID: whatsappID, To: to, ID: msgID, Emoji: emoji}
	var resp messageSendResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageReact, req, &resp); err != nil {
		return "", time.Time{}, translateRPCError(err)
	}
	return resp.ID, time.Unix(resp.Timestamp, 0), nil
}

func MarkRead(ctx context.Context, rpc RPCClient, whatsappID uint, chatJID, senderJID string, msgIDs []string, ts int64) *errors.AppError {
	req := markReadRequest{WhatsappID: whatsappID, ChatJID: chatJID, SenderJID: senderJID, MessageIDs: msgIDs, Timestamp: ts}
	var resp struct{}
	if err := rpc.Call(ctx, exchangeWaRPC, routingMessageMarkRead, req, &resp); err != nil {
		return translateRPCError(err)
	}
	return nil
}

func CheckNumber(ctx context.Context, rpc RPCClient, whatsappID uint, numbers []string) ([]CheckResult, *errors.AppError) {
	req := checkNumberRequest{WhatsappID: whatsappID, Numbers: numbers}
	var resp checkNumberResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingContactCheckNumber, req, &resp); err != nil {
		return nil, translateRPCError(err)
	}
	if resp.Results == nil {
		resp.Results = []CheckResult{}
	}
	return resp.Results, nil
}

func GetProfilePicURL(ctx context.Context, rpc RPCClient, whatsappID uint, jid string) (string, *errors.AppError) {
	req := profilePicRequest{WhatsappID: whatsappID, JID: jid}
	var resp profilePicResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingContactProfilePic, req, &resp); err != nil {
		return "", translateRPCError(err)
	}
	return resp.URL, nil
}

func GetContactInfo(ctx context.Context, rpc RPCClient, whatsappID uint, jids []string) ([]ContactInfo, *errors.AppError) {
	req := contactInfoRequest{WhatsappID: whatsappID, JIDs: jids}
	var resp contactInfoResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingContactInfo, req, &resp); err != nil {
		return nil, translateRPCError(err)
	}
	if resp.Infos == nil {
		resp.Infos = []ContactInfo{}
	}
	return resp.Infos, nil
}

func SendPresence(ctx context.Context, rpc RPCClient, whatsappID uint, presence string) *errors.AppError {
	req := presenceRequest{WhatsappID: whatsappID, Presence: presence}
	var resp struct{}
	if err := rpc.Call(ctx, exchangeWaRPC, routingPresenceSend, req, &resp); err != nil {
		return translateRPCError(err)
	}
	return nil
}

func SendChatPresence(ctx context.Context, rpc RPCClient, whatsappID uint, chatJID, state string) *errors.AppError {
	req := chatPresenceRequest{WhatsappID: whatsappID, ChatJID: chatJID, State: state}
	var resp struct{}
	if err := rpc.Call(ctx, exchangeWaRPC, routingChatPresenceSend, req, &resp); err != nil {
		return translateRPCError(err)
	}
	return nil
}

func PairPhone(ctx context.Context, rpc RPCClient, whatsappID uint, phone string) (string, *errors.AppError) {
	req := pairPhoneRequest{WhatsappID: whatsappID, Phone: phone}
	var resp pairPhoneResponse
	if err := rpc.Call(ctx, exchangeWaRPC, routingSessionPairPhone, req, &resp); err != nil {
		return "", translateRPCError(err)
	}
	return resp.LinkingCode, nil
}
