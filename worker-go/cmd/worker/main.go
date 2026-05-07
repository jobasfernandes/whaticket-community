package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jobasfernandes/whaticket-go-worker/internal/config"
	platlog "github.com/jobasfernandes/whaticket-go-worker/internal/platform/log"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited with error", slog.Any("err", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := platlog.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	logger.Info("worker bootstrap", slog.Any("config", cfg.Redacted()))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	<-ctx.Done()

	logger.Info("worker shutting down")
	return nil
}
