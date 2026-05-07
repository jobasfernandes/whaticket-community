package media

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"

	apperrors "github.com/jobasfernandes/whaticket-go-worker/internal/platform/errors"
)

func TestDecodeDataURLBase64(t *testing.T) {
	input := "data:image/png;base64,aGVsbG8="
	data, mime, err := DecodeDataURLOrFetch(context.Background(), input, 1024)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("got mime %q, want image/png", mime)
	}
	if string(data) != "hello" {
		t.Errorf("got data %q, want hello", data)
	}
}

func TestDecodeDataURLNoBase64ShouldFail(t *testing.T) {
	input := "data:text/plain,hello"
	_, _, err := DecodeDataURLOrFetch(context.Background(), input, 1024)
	if err == nil {
		t.Fatal("expected error for non-base64 data url")
	}
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) || appErr.Code != apperrors.ErrMediaDecode {
		t.Errorf("expected AppError with ErrMediaDecode, got %v", err)
	}
}

func TestDecodeGarbage(t *testing.T) {
	_, _, err := DecodeDataURLOrFetch(context.Background(), "not-a-url", 1024)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) || appErr.Code != apperrors.ErrMediaDecode {
		t.Errorf("expected AppError ErrMediaDecode, got %v", err)
	}
}

func TestDecodeEmpty(t *testing.T) {
	_, _, err := DecodeDataURLOrFetch(context.Background(), "", 1024)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("imagebytes"))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, mime, err := DecodeDataURLOrFetch(ctx, srv.URL+"/img.jpg", 1024)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(data) != "imagebytes" {
		t.Errorf("got data %q, want imagebytes", data)
	}
	if mime != "image/jpeg" {
		t.Errorf("got mime %q", mime)
	}
}

func TestFetchURL404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, _, err := DecodeDataURLOrFetch(context.Background(), srv.URL+"/missing", 1024)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) || appErr.Code != apperrors.ErrMediaDecode {
		t.Errorf("expected AppError ErrMediaDecode, got %v", err)
	}
}

func TestFetchURLLimit(t *testing.T) {
	body := strings.Repeat("a", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	data, _, err := DecodeDataURLOrFetch(context.Background(), srv.URL, 100)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(data) != 100 {
		t.Errorf("expected 100 bytes (limit), got %d", len(data))
	}
}

func TestWAMediaType(t *testing.T) {
	cases := []struct {
		kind string
		want whatsmeow.MediaType
	}{
		{kind: "image", want: whatsmeow.MediaImage},
		{kind: "audio", want: whatsmeow.MediaAudio},
		{kind: "video", want: whatsmeow.MediaVideo},
		{kind: "document", want: whatsmeow.MediaDocument},
		{kind: "sticker", want: whatsmeow.MediaImage},
		{kind: "IMAGE", want: whatsmeow.MediaImage},
	}
	for _, tc := range cases {
		got, err := WAMediaType(tc.kind)
		if err != nil {
			t.Errorf("WAMediaType(%q) err: %v", tc.kind, err)
			continue
		}
		if got != tc.want {
			t.Errorf("WAMediaType(%q) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestWAMediaTypeUnknown(t *testing.T) {
	_, err := WAMediaType("widget")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) || appErr.Code != apperrors.ErrMediaDecode {
		t.Errorf("expected AppError ErrMediaDecode, got %v", err)
	}
}
