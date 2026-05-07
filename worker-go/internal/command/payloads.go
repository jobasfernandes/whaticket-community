package command

import (
	whatsmeowpkg "github.com/jobasfernandes/whaticket-go-worker/internal/whatsmeow"
)

type ContextInfo struct {
	StanzaID    string `json:"stanzaId"`
	Participant string `json:"participant,omitempty"`
}

type SendResp struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

type SessionStartReq struct {
	WhatsappID       int                           `json:"whatsappId"`
	JID              string                        `json:"jid,omitempty"`
	AdvancedSettings whatsmeowpkg.AdvancedSettings `json:"advancedSettings"`
	MediaMode        string                        `json:"mediaMode"`
	ProxyURL         string                        `json:"proxyURL,omitempty"`
}

type SessionStopReq struct {
	WhatsappID int `json:"whatsappId"`
}

type SessionLogoutReq struct {
	WhatsappID int `json:"whatsappId"`
}

type SessionUpdateSettingsReq struct {
	WhatsappID       int                           `json:"whatsappId"`
	AdvancedSettings whatsmeowpkg.AdvancedSettings `json:"advancedSettings"`
	MediaMode        string                        `json:"mediaMode"`
}

type PairPhoneReq struct {
	WhatsappID int    `json:"whatsappId"`
	Phone      string `json:"phone"`
}

type PairPhoneResp struct {
	Code string `json:"code"`
}

type SendTextReq struct {
	WhatsappID  int          `json:"whatsappId"`
	To          string       `json:"to"`
	Body        string       `json:"body"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type SendImageReq struct {
	WhatsappID  int          `json:"whatsappId"`
	To          string       `json:"to"`
	Image       string       `json:"image"`
	Caption     string       `json:"caption,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type SendAudioReq struct {
	WhatsappID  int          `json:"whatsappId"`
	To          string       `json:"to"`
	Audio       string       `json:"audio"`
	PTT         *bool        `json:"ptt,omitempty"`
	Seconds     uint32       `json:"seconds,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type SendVideoReq struct {
	WhatsappID  int          `json:"whatsappId"`
	To          string       `json:"to"`
	Video       string       `json:"video"`
	Caption     string       `json:"caption,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type SendDocumentReq struct {
	WhatsappID  int          `json:"whatsappId"`
	To          string       `json:"to"`
	Document    string       `json:"document"`
	FileName    string       `json:"fileName"`
	Caption     string       `json:"caption,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	ContextInfo *ContextInfo `json:"contextInfo,omitempty"`
}

type SendStickerReq struct {
	WhatsappID int    `json:"whatsappId"`
	To         string `json:"to"`
	Sticker    string `json:"sticker"`
}

type SendContactReq struct {
	WhatsappID int    `json:"whatsappId"`
	To         string `json:"to"`
	Name       string `json:"name"`
	Vcard      string `json:"vcard"`
}

type SendLocationReq struct {
	WhatsappID int     `json:"whatsappId"`
	To         string  `json:"to"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Name       string  `json:"name,omitempty"`
}

type DeleteMessageReq struct {
	WhatsappID int    `json:"whatsappId"`
	To         string `json:"to"`
	ID         string `json:"id"`
}

type EditMessageReq struct {
	WhatsappID int    `json:"whatsappId"`
	To         string `json:"to"`
	ID         string `json:"id"`
	Body       string `json:"body"`
}

type ReactMessageReq struct {
	WhatsappID  int    `json:"whatsappId"`
	To          string `json:"to"`
	ID          string `json:"id"`
	FromMe      bool   `json:"fromMe"`
	Participant string `json:"participant,omitempty"`
	Emoji       string `json:"emoji"`
}

type MarkReadReq struct {
	WhatsappID  int      `json:"whatsappId"`
	ChatJID     string   `json:"chatJid"`
	SenderJID   string   `json:"senderJid,omitempty"`
	IDs         []string `json:"ids"`
	TimestampMS int64    `json:"timestampMs"`
}

type CheckNumberReq struct {
	WhatsappID int      `json:"whatsappId"`
	Numbers    []string `json:"numbers"`
}

type CheckNumberResult struct {
	Number string `json:"number"`
	Exists bool   `json:"exists"`
	JID    string `json:"jid,omitempty"`
}

type GetProfilePicReq struct {
	WhatsappID int    `json:"whatsappId"`
	JID        string `json:"jid"`
	Preview    bool   `json:"preview,omitempty"`
}

type GetProfilePicResp struct {
	URL string `json:"url"`
}

type GetContactInfoReq struct {
	WhatsappID int      `json:"whatsappId"`
	JIDs       []string `json:"jids"`
}

type ContactInfo struct {
	JID          string `json:"jid"`
	PushName     string `json:"pushName,omitempty"`
	BusinessName string `json:"businessName,omitempty"`
	Status       string `json:"status,omitempty"`
}

type SendPresenceReq struct {
	WhatsappID int    `json:"whatsappId"`
	Presence   string `json:"presence"`
}

type SendChatPresenceReq struct {
	WhatsappID int    `json:"whatsappId"`
	ChatJID    string `json:"chatJid"`
	State      string `json:"state"`
}
