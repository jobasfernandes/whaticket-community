package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func TestConvertToStickerFromPNG(t *testing.T) {
	src := buildTestImage(120, 80, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	out, mime, animated, err := ConvertToSticker(buf.Bytes())
	if err != nil {
		t.Fatalf("ConvertToSticker: %v", err)
	}
	if mime != "image/webp" {
		t.Errorf("mime = %q, want image/webp", mime)
	}
	if animated {
		t.Error("static png should not be animated")
	}
	if len(out) == 0 {
		t.Fatal("output bytes empty")
	}
	if len(out) > stickerMaxBytes {
		t.Errorf("output size %d exceeds limit %d", len(out), stickerMaxBytes)
	}
	if !bytes.HasPrefix(out, []byte("RIFF")) {
		t.Error("output is not a RIFF WebP container")
	}
}

func TestConvertToStickerWebPPassthrough(t *testing.T) {
	src := buildTestImage(40, 40, color.RGBA{R: 0, G: 200, B: 0, A: 255})
	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, src, &nativewebp.Options{}); err != nil {
		t.Fatalf("encode webp: %v", err)
	}
	if buf.Len() > stickerMaxBytes {
		t.Skipf("test webp unexpectedly larger than max (%d bytes)", buf.Len())
	}

	out, mime, _, err := ConvertToSticker(buf.Bytes())
	if err != nil {
		t.Fatalf("ConvertToSticker: %v", err)
	}
	if mime != "image/webp" {
		t.Errorf("mime = %q, want image/webp", mime)
	}
	if !bytes.Equal(out, buf.Bytes()) {
		t.Error("expected pass-through bytes for compliant webp input")
	}
}

func TestConvertToStickerInvalidInput(t *testing.T) {
	_, _, _, err := ConvertToSticker(nil)
	if !errors.Is(err, ErrStickerEmptyInput) {
		t.Errorf("nil input err = %v, want ErrStickerEmptyInput", err)
	}

	_, _, _, err = ConvertToSticker([]byte{0x00, 0x01, 0x02, 0x03, 0x04})
	if err == nil {
		t.Error("expected error decoding garbage bytes")
	}
}

func TestBuildStickerCanvasAspectRatio(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	rgba := buildStickerCanvas(src, 512)

	bounds := rgba.Bounds()
	if bounds.Dx() != 512 || bounds.Dy() != 512 {
		t.Errorf("canvas size = %dx%d, want 512x512", bounds.Dx(), bounds.Dy())
	}

	corner := rgba.RGBAAt(0, 0)
	if corner.A != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent padding)", corner.A)
	}
}

func buildTestImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}
