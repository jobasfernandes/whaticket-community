package media

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

const (
	KindImage    = "image"
	KindAudio    = "audio"
	KindVideo    = "video"
	KindDocument = "document"
	KindSticker  = "sticker"
)

const (
	ModeS3 = "s3"
)

type Uploader interface {
	Upload(ctx context.Context, objectKey string, data []byte, mimeType string) (string, error)
}

type Downloadable struct {
	Msg      whatsmeow.DownloadableMessage
	Mime     string
	Fallback string
	Kind     string
	Animated bool
}

type IncomingPayload struct {
	Event           any    `json:"event"`
	MimeType        string `json:"mimeType,omitempty"`
	FileName        string `json:"fileName,omitempty"`
	MediaURL        string `json:"mediaUrl,omitempty"`
	S3Key           string `json:"s3Key,omitempty"`
	Base64          string `json:"base64,omitempty"`
	IsSticker       bool   `json:"isSticker,omitempty"`
	StickerAnimated bool   `json:"stickerAnimated,omitempty"`
}

func FirstMedia(msg *waE2E.Message) *Downloadable {
	if msg == nil {
		return nil
	}
	if img := msg.GetImageMessage(); img != nil {
		return &Downloadable{Msg: img, Mime: img.GetMimetype(), Fallback: ".jpg", Kind: KindImage}
	}
	if audio := msg.GetAudioMessage(); audio != nil {
		return &Downloadable{Msg: audio, Mime: audio.GetMimetype(), Fallback: ".ogg", Kind: KindAudio}
	}
	if video := msg.GetVideoMessage(); video != nil {
		return &Downloadable{Msg: video, Mime: video.GetMimetype(), Fallback: ".mp4", Kind: KindVideo}
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		fallback := ".bin"
		if name := doc.GetFileName(); name != "" {
			if ext := filepath.Ext(name); ext != "" {
				fallback = ext
			}
		}
		return &Downloadable{Msg: doc, Mime: doc.GetMimetype(), Fallback: fallback, Kind: KindDocument}
	}
	if sticker := msg.GetStickerMessage(); sticker != nil {
		return &Downloadable{
			Msg:      sticker,
			Mime:     sticker.GetMimetype(),
			Fallback: ".webp",
			Kind:     KindSticker,
			Animated: sticker.GetIsAnimated(),
		}
	}
	return nil
}

func SafeExtension(mimeType, fallback string) string {
	if fallback == "" {
		fallback = ".bin"
	} else if !strings.HasPrefix(fallback, ".") {
		fallback = "." + fallback
	}
	if mimeType == "" {
		return fallback
	}
	if ext, ok := wellKnownMimeExt[strings.ToLower(strings.TrimSpace(mimeType))]; ok {
		return ext
	}
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return fallback
	}
	return exts[0]
}

var wellKnownMimeExt = map[string]string{
	"image/jpeg":               ".jpg",
	"image/jpg":                ".jpg",
	"image/png":                ".png",
	"image/gif":                ".gif",
	"image/webp":               ".webp",
	"image/bmp":                ".bmp",
	"image/heic":               ".heic",
	"audio/ogg":                ".ogg",
	"audio/mpeg":               ".mp3",
	"audio/mp4":                ".m4a",
	"audio/aac":                ".aac",
	"audio/wav":                ".wav",
	"audio/x-wav":              ".wav",
	"audio/opus":               ".opus",
	"video/mp4":                ".mp4",
	"video/quicktime":          ".mov",
	"video/webm":               ".webm",
	"video/3gpp":               ".3gp",
	"application/pdf":          ".pdf",
	"application/zip":          ".zip",
	"application/x-zip":        ".zip",
	"application/octet-stream": ".bin",
}

func ProcessIncoming(ctx context.Context, client *whatsmeow.Client, connID int, evt *events.Message, store Uploader, _ string, log *slog.Logger) (IncomingPayload, error) {
	if log == nil {
		log = slog.Default()
	}
	payload := IncomingPayload{Event: evt}
	if evt == nil || evt.Message == nil {
		return payload, nil
	}

	dl := FirstMedia(evt.Message)
	if dl == nil {
		return payload, nil
	}

	data, err := client.Download(ctx, dl.Msg)
	if err != nil {
		log.Warn("download media failed",
			slog.String("kind", dl.Kind),
			slog.String("msg_id", evt.Info.ID),
			slog.Any("err", err),
		)
		return payload, nil
	}

	payload.MimeType = dl.Mime
	payload.FileName = evt.Info.ID + SafeExtension(dl.Mime, dl.Fallback)

	chatUser := evt.Info.Chat.User
	objectKey := fmt.Sprintf("wa-media/%d/%s/%s", connID, chatUser, payload.FileName)

	uploadIfPossible(ctx, store, objectKey, data, dl.Mime, &payload, log)

	if dl.Kind == KindSticker {
		payload.IsSticker = true
		payload.StickerAnimated = dl.Animated
	}

	return payload, nil
}

func uploadIfPossible(ctx context.Context, store Uploader, objectKey string, data []byte, mimeType string, payload *IncomingPayload, log *slog.Logger) {
	if store == nil {
		log.Warn("media store not configured, skipping upload",
			slog.String("object_key", objectKey),
		)
		return
	}
	publicURL, err := store.Upload(ctx, objectKey, data, mimeType)
	if err != nil {
		log.Error("media upload failed",
			slog.String("object_key", objectKey),
			slog.Any("err", err),
		)
		return
	}
	payload.MediaURL = publicURL
	payload.S3Key = objectKey
}
