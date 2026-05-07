package media

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/jobasfernandes/whaticket-go-worker/internal/config"
)

type MinioClient struct {
	client    *minio.Client
	bucket    string
	endpoint  string
	useSSL    bool
	publicURL string
	log       *slog.Logger
}

func NewMinioClient(cfg *config.Config, log *slog.Logger) (*MinioClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("media: config is nil")
	}
	if cfg.S3Endpoint == "" {
		return nil, fmt.Errorf("media: S3 endpoint is empty")
	}
	if log == nil {
		log = slog.Default()
	}

	cli, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("media: minio.New: %w", err)
	}

	return &MinioClient{
		client:    cli,
		bucket:    cfg.S3Bucket,
		endpoint:  cfg.S3Endpoint,
		useSSL:    cfg.S3UseSSL,
		publicURL: strings.TrimRight(cfg.S3PublicURL, "/"),
		log:       log,
	}, nil
}

func (m *MinioClient) Upload(ctx context.Context, objectKey string, data []byte, mimeType string) (string, error) {
	if m == nil || m.client == nil {
		return "", fmt.Errorf("media: minio client not initialized")
	}
	reader := bytes.NewReader(data)
	_, err := m.client.PutObject(ctx, m.bucket, objectKey, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: mimeType,
	})
	if err != nil {
		return "", fmt.Errorf("media: put object: %w", err)
	}
	return m.buildPublicURL(objectKey), nil
}

func (m *MinioClient) Health(ctx context.Context) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("media: minio client not initialized")
	}
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("media: bucket exists check: %w", err)
	}
	if !exists {
		return fmt.Errorf("media: bucket %q does not exist", m.bucket)
	}
	return nil
}

func (m *MinioClient) Bucket() string {
	return m.bucket
}

func (m *MinioClient) buildPublicURL(objectKey string) string {
	if m.publicURL != "" {
		return fmt.Sprintf("%s/%s/%s", m.publicURL, m.bucket, objectKey)
	}
	scheme := "http"
	if m.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, m.endpoint, m.bucket, objectKey)
}
