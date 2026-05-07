package media

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"

	apperrors "github.com/jobasfernandes/whaticket-go-worker/internal/platform/errors"
)

const defaultOutboundFetchTimeout = 30 * time.Second

func DecodeDataURLOrFetch(ctx context.Context, input string, maxBytes int64) ([]byte, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, "", apperrors.New(apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	if strings.HasPrefix(input, "data:") {
		return decodeDataURL(input)
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return fetchURL(ctx, input, maxBytes)
	}
	return nil, "", apperrors.New(apperrors.ErrMediaDecode, http.StatusBadRequest)
}

func decodeDataURL(input string) ([]byte, string, error) {
	rest := strings.TrimPrefix(input, "data:")
	commaIdx := strings.Index(rest, ",")
	if commaIdx < 0 {
		return nil, "", apperrors.New(apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	meta := rest[:commaIdx]
	payload := rest[commaIdx+1:]

	mimeType := ""
	isBase64 := false
	for _, part := range strings.Split(meta, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "base64" {
			isBase64 = true
			continue
		}
		if mimeType == "" {
			mimeType = part
		}
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	if !isBase64 {
		return nil, "", apperrors.New(apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", apperrors.Wrap(err, apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	return data, mimeType, nil
}

func fetchURL(ctx context.Context, url string, maxBytes int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", apperrors.Wrap(err, apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
	httpClient := &http.Client{Timeout: defaultOutboundFetchTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", apperrors.Wrap(err, apperrors.ErrMediaDecode, http.StatusBadGateway)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", apperrors.New(apperrors.ErrMediaDecode, resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if maxBytes > 0 {
		reader = io.LimitReader(resp.Body, maxBytes)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", apperrors.Wrap(err, apperrors.ErrMediaDecode, http.StatusBadGateway)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func UploadOutbound(ctx context.Context, client *whatsmeow.Client, data []byte, mediaType whatsmeow.MediaType) (*whatsmeow.UploadResponse, error) {
	if client == nil {
		return nil, apperrors.New(apperrors.ErrMediaUpload, http.StatusInternalServerError)
	}
	resp, err := client.Upload(ctx, data, mediaType)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrMediaUpload, http.StatusBadGateway)
	}
	return &resp, nil
}

func WAMediaType(kind string) (whatsmeow.MediaType, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindImage:
		return whatsmeow.MediaImage, nil
	case KindAudio:
		return whatsmeow.MediaAudio, nil
	case KindVideo:
		return whatsmeow.MediaVideo, nil
	case KindDocument:
		return whatsmeow.MediaDocument, nil
	case KindSticker:
		return whatsmeow.MediaImage, nil
	default:
		return "", apperrors.Wrap(fmt.Errorf("unknown media kind %q", kind), apperrors.ErrMediaDecode, http.StatusBadRequest)
	}
}
