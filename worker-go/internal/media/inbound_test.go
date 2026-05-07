package media

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestFirstMediaImage(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Mimetype: proto.String("image/jpeg"),
		},
	}
	dl := FirstMedia(msg)
	if dl == nil {
		t.Fatal("expected image downloadable")
	}
	if dl.Kind != KindImage || dl.Mime != "image/jpeg" || dl.Fallback != ".jpg" {
		t.Errorf("unexpected downloadable %+v", dl)
	}
}

func TestFirstMediaAudio(t *testing.T) {
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			Mimetype: proto.String("audio/ogg"),
		},
	}
	dl := FirstMedia(msg)
	if dl == nil || dl.Kind != KindAudio || dl.Fallback != ".ogg" {
		t.Errorf("unexpected: %+v", dl)
	}
}

func TestFirstMediaVideo(t *testing.T) {
	msg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Mimetype: proto.String("video/mp4"),
		},
	}
	dl := FirstMedia(msg)
	if dl == nil || dl.Kind != KindVideo || dl.Fallback != ".mp4" {
		t.Errorf("unexpected: %+v", dl)
	}
}

func TestFirstMediaDocumentWithFileName(t *testing.T) {
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Mimetype: proto.String("application/pdf"),
			FileName: proto.String("invoice.pdf"),
		},
	}
	dl := FirstMedia(msg)
	if dl == nil || dl.Kind != KindDocument {
		t.Fatalf("unexpected: %+v", dl)
	}
	if dl.Fallback != ".pdf" {
		t.Errorf("expected fallback .pdf, got %q", dl.Fallback)
	}
}

func TestFirstMediaDocumentNoFileName(t *testing.T) {
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Mimetype: proto.String("application/octet-stream"),
		},
	}
	dl := FirstMedia(msg)
	if dl == nil {
		t.Fatal("expected downloadable")
	}
	if dl.Fallback != ".bin" {
		t.Errorf("expected fallback .bin, got %q", dl.Fallback)
	}
}

func TestFirstMediaSticker(t *testing.T) {
	msg := &waE2E.Message{
		StickerMessage: &waE2E.StickerMessage{
			Mimetype:   proto.String("image/webp"),
			IsAnimated: proto.Bool(true),
		},
	}
	dl := FirstMedia(msg)
	if dl == nil {
		t.Fatal("expected sticker downloadable")
	}
	if dl.Kind != KindSticker || !dl.Animated || dl.Fallback != ".webp" {
		t.Errorf("unexpected: %+v", dl)
	}
}

func TestFirstMediaPlainText(t *testing.T) {
	msg := &waE2E.Message{Conversation: proto.String("hello")}
	if dl := FirstMedia(msg); dl != nil {
		t.Errorf("expected nil for plain text, got %+v", dl)
	}
}

func TestFirstMediaNil(t *testing.T) {
	if dl := FirstMedia(nil); dl != nil {
		t.Errorf("expected nil for nil msg, got %+v", dl)
	}
}

func TestSafeExtension(t *testing.T) {
	cases := []struct {
		mime     string
		fallback string
		want     string
	}{
		{mime: "image/jpeg", fallback: ".jpg", want: ".jpg"},
		{mime: "", fallback: ".bin", want: ".bin"},
		{mime: "", fallback: "", want: ".bin"},
		{mime: "x-unknown/none", fallback: ".dat", want: ".dat"},
		{mime: "application/pdf", fallback: ".bin", want: ".pdf"},
		{mime: "", fallback: "ogg", want: ".ogg"},
	}
	for _, tc := range cases {
		got := SafeExtension(tc.mime, tc.fallback)
		if got != tc.want {
			t.Errorf("SafeExtension(%q,%q) = %q, want %q", tc.mime, tc.fallback, got, tc.want)
		}
	}
}
