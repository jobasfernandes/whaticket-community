//go:build integration

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/jobasfernandes/whaticket-go-worker/internal/testenv"
)

func TestWorkerBootSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rmqEnv := testenv.StartRabbitMQ(ctx, t)
	minioEnv := testenv.StartMinIO(ctx, t)

	port := freePort(t)
	storePath := filepath.Join(t.TempDir(), "whatsmeow.db")

	t.Setenv("RABBITMQ_URL", rmqEnv.URL)
	t.Setenv("WORKER_S3_ENDPOINT", minioEnv.Endpoint)
	t.Setenv("WORKER_S3_ACCESS_KEY", minioEnv.AccessKey)
	t.Setenv("WORKER_S3_SECRET_KEY", minioEnv.SecretKey)
	t.Setenv("WORKER_S3_BUCKET", minioEnv.Bucket)
	t.Setenv("WORKER_S3_USE_SSL", "false")
	t.Setenv("WORKER_SQLSTORE_PATH", storePath)
	t.Setenv("WORKER_HEALTH_PORT", port)
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("LOG_FORMAT", "text")

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	exited := make(chan error, 1)
	go func() {
		exited <- Run(runCtx)
	}()

	healthURL := "http://127.0.0.1:" + port + "/health"
	deadline := time.Now().Add(15 * time.Second)
	client := &http.Client{Timeout: 1 * time.Second}
	var lastErr error
	healthy := false
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(runCtx, http.MethodGet, healthURL, nil)
		if reqErr != nil {
			lastErr = reqErr
			break
		}
		resp, doErr := client.Do(req)
		if doErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				healthy = true
				lastErr = nil
				break
			}
			lastErr = errors.New("non-200 from /health: " + resp.Status)
		} else {
			lastErr = doErr
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !healthy {
		runCancel()
		<-exited
		t.Fatalf("/health did not return 200 within 15s: %v", lastErr)
	}

	runCancel()

	select {
	case err := <-exited:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Run exited with error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not exit within 30s after context cancel")
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = l.Close() }()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split host:port: %v", err)
	}
	return port
}
