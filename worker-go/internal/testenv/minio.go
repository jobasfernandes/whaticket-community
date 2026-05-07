//go:build integration

package testenv

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/jobasfernandes/whaticket-go-worker/internal/config"
	"github.com/jobasfernandes/whaticket-go-worker/internal/media"
)

const (
	minioImage     = "minio/minio:RELEASE.2024-01-16T16-07-38Z"
	minioStartTime = 90 * time.Second
	defaultBucket  = "whaticket-media"
)

type MinIO struct {
	Container *tcminio.MinioContainer
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Client    *media.MinioClient
	Config    *config.Config
}

func StartMinIO(ctx context.Context, t *testing.T) *MinIO {
	t.Helper()

	startCtx, cancel := context.WithTimeout(ctx, minioStartTime)
	defer cancel()

	container, err := tcminio.Run(startCtx, minioImage)
	if err != nil {
		t.Skipf("minio testcontainer unavailable: %v", err)
		return nil
	}

	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	connStr, err := container.ConnectionString(startCtx)
	if err != nil {
		t.Fatalf("minio connection string: %v", err)
	}
	endpoint := strings.TrimPrefix(strings.TrimPrefix(connStr, "https://"), "http://")

	if err := ensureBucket(startCtx, endpoint, container.Username, container.Password, defaultBucket); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	cfg := &config.Config{
		S3Endpoint:  endpoint,
		S3AccessKey: container.Username,
		S3SecretKey: container.Password,
		S3Bucket:    defaultBucket,
		S3UseSSL:    false,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := media.NewMinioClient(cfg, logger)
	if err != nil {
		t.Fatalf("media.NewMinioClient: %v", err)
	}

	return &MinIO{
		Container: container,
		Endpoint:  endpoint,
		AccessKey: container.Username,
		SecretKey: container.Password,
		Bucket:    defaultBucket,
		Client:    client,
		Config:    cfg,
	}
}

func ensureBucket(ctx context.Context, endpoint, accessKey, secretKey, bucket string) error {
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}
	exists, err := cli.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return cli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}
