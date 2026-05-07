package linkpreview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleHTML = `<!DOCTYPE html>
<html>
<head>
<title>Plain Title</title>
<meta property="og:title" content="OG Title" />
<meta property="og:description" content="OG Description" />
<meta property="og:image" content="/cover.png" />
<meta name="description" content="Plain description fallback" />
</head>
<body><h1>hi</h1></body>
</html>`

func TestExtractFromOpenGraph(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(sampleHTML))
	})
	mux.HandleFunc("/cover.png", func(w http.ResponseWriter, r *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 200, 200))
		for y := 0; y < 200; y++ {
			for x := 0; x < 200; x++ {
				img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
			}
		}
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := "checa essa pagina " + srv.URL + "/page legal hein"
	preview, ok := Extract(context.Background(), body, 5*time.Second)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if preview == nil {
		t.Fatal("preview is nil")
	}
	if preview.Title != "OG Title" {
		t.Errorf("title = %q, want OG Title", preview.Title)
	}
	if preview.Description != "OG Description" {
		t.Errorf("description = %q, want OG Description", preview.Description)
	}
	if preview.URL != srv.URL+"/page" {
		t.Errorf("url = %q, want %s/page", preview.URL, srv.URL)
	}
	if len(preview.Thumbnail) == 0 {
		t.Error("expected thumbnail bytes")
	}
	if preview.ThumbnailMime != "image/jpeg" {
		t.Errorf("thumbnail mime = %q, want image/jpeg", preview.ThumbnailMime)
	}
}

func TestExtractFallbackToTitleTag(t *testing.T) {
	html := `<html><head><title>Solo Title</title></head><body></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	preview, ok := Extract(context.Background(), srv.URL, 3*time.Second)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if preview.Title != "Solo Title" {
		t.Errorf("title = %q, want Solo Title", preview.Title)
	}
	if preview.Description != "" {
		t.Errorf("description = %q, want empty", preview.Description)
	}
	if len(preview.Thumbnail) != 0 {
		t.Errorf("thumbnail should be empty when no image meta is present")
	}
}

func TestExtractNoURL(t *testing.T) {
	preview, ok := Extract(context.Background(), "no link here", time.Second)
	if ok {
		t.Error("expected ok=false when body has no URL")
	}
	if preview != nil {
		t.Error("expected nil preview")
	}
}

func TestExtractNoTitleReturnsFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head></head><body></body></html>`))
	}))
	defer srv.Close()

	preview, ok := Extract(context.Background(), srv.URL, time.Second)
	if ok {
		t.Errorf("expected ok=false when no title found, got preview=%+v", preview)
	}
}

func TestExtractTimeoutDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	_, ok := Extract(context.Background(), srv.URL, 50*time.Millisecond)
	if ok {
		t.Error("expected timeout to result in ok=false")
	}
}

func TestFirstURLTrimsTrailing(t *testing.T) {
	cases := map[string]string{
		"check https://example.com.":         "https://example.com",
		"link http://foo.bar/path?x=1, plus": "http://foo.bar/path?x=1",
		"hello world":                        "",
		"https://example.com/path)":          "https://example.com/path",
	}
	for input, want := range cases {
		if got := firstURL(input); got != want {
			t.Errorf("firstURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFetchBytesRespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := strings.Repeat("a", 4096)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	out, err := fetchBytes(context.Background(), srv.URL, 256)
	if err != nil {
		t.Fatalf("fetchBytes: %v", err)
	}
	if len(out) != 256 {
		t.Errorf("limit not respected, got %d bytes", len(out))
	}
	if !bytes.Equal(out, bytes.Repeat([]byte{'a'}, 256)) {
		t.Error("unexpected content")
	}
}
