package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	stickerCanvasPrimary   = 512
	stickerCanvasFallback  = 256
	stickerMaxBytes        = 100 * 1024
	stickerDetectSniffSize = 512
)

var (
	ErrStickerEmptyInput  = errors.New("sticker: empty input")
	ErrStickerUnsupported = errors.New("sticker: unsupported input format")
	ErrStickerOversized   = errors.New("sticker: encoded output exceeds size limit after fallback")
)

func ConvertToSticker(input []byte) ([]byte, string, bool, error) {
	if len(input) == 0 {
		return nil, "", false, ErrStickerEmptyInput
	}

	mime := detectStickerMime(input)

	if mime == "image/webp" && len(input) <= stickerMaxBytes {
		return input, "image/webp", false, nil
	}

	src, err := decodeSticker(input, mime)
	if err != nil {
		return nil, "", false, err
	}

	out, err := encodeStickerAt(src, stickerCanvasPrimary)
	if err != nil {
		return nil, "", false, err
	}
	if len(out) <= stickerMaxBytes {
		return out, "image/webp", false, nil
	}

	out, err = encodeStickerAt(src, stickerCanvasFallback)
	if err != nil {
		return nil, "", false, err
	}
	if len(out) <= stickerMaxBytes {
		return out, "image/webp", false, nil
	}

	return nil, "", false, fmt.Errorf("%w: %d bytes", ErrStickerOversized, len(out))
}

func detectStickerMime(input []byte) string {
	sniff := input
	if len(sniff) > stickerDetectSniffSize {
		sniff = sniff[:stickerDetectSniffSize]
	}
	return http.DetectContentType(sniff)
}

func decodeSticker(input []byte, mime string) (image.Image, error) {
	switch mime {
	case "image/png":
		return png.Decode(bytes.NewReader(input))
	case "image/jpeg", "image/jpg":
		return jpeg.Decode(bytes.NewReader(input))
	case "image/gif":
		return gif.Decode(bytes.NewReader(input))
	case "image/webp":
		img, _, err := image.Decode(bytes.NewReader(input))
		return img, err
	default:
		img, _, err := image.Decode(bytes.NewReader(input))
		if err != nil {
			return nil, fmt.Errorf("%w: mime %s", ErrStickerUnsupported, mime)
		}
		return img, nil
	}
}

func encodeStickerAt(src image.Image, canvas int) ([]byte, error) {
	rgba := buildStickerCanvas(src, canvas)
	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, rgba, &nativewebp.Options{UseExtendedFormat: true}); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}
	return buf.Bytes(), nil
}

func buildStickerCanvas(src image.Image, canvas int) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	dstRect := image.Rect(0, 0, canvas, canvas)
	rgba := image.NewRGBA(dstRect)

	if srcW <= 0 || srcH <= 0 {
		return rgba
	}

	scaleW := float64(canvas) / float64(srcW)
	scaleH := float64(canvas) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	targetW := int(float64(srcW) * scale)
	targetH := int(float64(srcH) * scale)
	if targetW <= 0 {
		targetW = 1
	}
	if targetH <= 0 {
		targetH = 1
	}

	offsetX := (canvas - targetW) / 2
	offsetY := (canvas - targetH) / 2
	target := image.Rect(offsetX, offsetY, offsetX+targetW, offsetY+targetH)
	draw.CatmullRom.Scale(rgba, target, src, bounds, draw.Over, nil)
	return rgba
}
