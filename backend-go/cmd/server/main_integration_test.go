//go:build integration

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jobasfernandes/whaticket-go-backend/internal/db/dbtest"
	"github.com/jobasfernandes/whaticket-go-backend/internal/rmqtest"
)

func TestServerBootSmoke(t *testing.T) {
	ctx := context.Background()

	pg := dbtest.StartPostgres(ctx, t)
	mq := rmqtest.StartRabbitMQ(ctx, t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := listener.Addr().String()
	if cerr := listener.Close(); cerr != nil {
		t.Fatalf("close port reservation: %v", cerr)
	}

	cfg := appConfig{
		ListenAddr:    addr,
		DatabaseDSN:   pg.DSN,
		RMQURL:        mq.URL,
		FrontendURL:   "http://localhost:3000",
		AccessSecret:  dbtest.TestAccessSecret,
		RefreshSecret: dbtest.TestRefreshSecret,
		LogLevel:      "error",
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	exited := make(chan error, 1)
	go func() {
		exited <- Run(runCtx, cfg)
	}()

	healthURL := "http://" + addr + "/health"
	deadline := time.Now().Add(10 * time.Second)
	client := &http.Client{Timeout: 1 * time.Second}
	var lastErr error
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
				lastErr = nil
				break
			}
			lastErr = errors.New("non-200 from /health: " + resp.Status)
		} else {
			lastErr = doErr
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr != nil {
		cancel()
		<-exited
		t.Fatalf("/health never returned 200: %v", lastErr)
	}

	cancel()

	select {
	case err := <-exited:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Run exited with error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not exit within 30s after context cancel")
	}
}
