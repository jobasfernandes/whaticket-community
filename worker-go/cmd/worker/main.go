package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/jobasfernandes/whaticket-go-worker/internal/command"
	"github.com/jobasfernandes/whaticket-go-worker/internal/config"
	"github.com/jobasfernandes/whaticket-go-worker/internal/health"
	"github.com/jobasfernandes/whaticket-go-worker/internal/media"
	platlog "github.com/jobasfernandes/whaticket-go-worker/internal/platform/log"
	"github.com/jobasfernandes/whaticket-go-worker/internal/rmq"
	whatsmeowpkg "github.com/jobasfernandes/whaticket-go-worker/internal/whatsmeow"
)

const (
	rmqStartTimeout      = 30 * time.Second
	shutdownGracePeriod  = 30 * time.Second
	deviceMetaPathSuffix = ".meta"
)

func main() {
	rootCtx, rootCancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer rootCancel()

	if err := Run(rootCtx); err != nil {
		slog.Error("worker exited with error", slog.Any("err", err))
		os.Exit(1)
	}
}

func Run(rootCtx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := platlog.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	logger.Info("worker bootstrap", slog.Any("config", cfg.Redacted()))

	openCtx, openCancel := context.WithTimeout(rootCtx, rmqStartTimeout)
	defer openCancel()

	container, err := whatsmeowpkg.OpenContainer(openCtx, cfg, waLog.Noop)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := container.Close(); cerr != nil {
			logger.Warn("container close failed", slog.Any("err", cerr))
		}
	}()

	whatsmeowpkg.SetGlobalDeviceProps(cfg)

	deviceMeta, err := whatsmeowpkg.OpenDeviceMeta(cfg.SQLStorePath + deviceMetaPathSuffix)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := deviceMeta.Close(); cerr != nil {
			logger.Warn("device meta close failed", slog.Any("err", cerr))
		}
	}()

	rmqClient := rmq.New(rmq.Config{URL: cfg.RMQUrl, Logger: logger})
	rmqClient.SetRole(rmq.RoleWorker)
	if err := rmqClient.Start(openCtx); err != nil {
		return err
	}

	minioClient, err := media.NewMinioClient(cfg, logger)
	if err != nil {
		return err
	}

	mgr := whatsmeowpkg.NewManager(container, logger)
	eventHandler := whatsmeowpkg.NewEventHandler(mgr, rmqClient, minioClient, logger)
	runtime := whatsmeowpkg.SessionRuntime{
		Container:    container,
		EventHandler: eventHandler,
		DeviceMeta:   deviceMeta,
		WALog:        waLog.Noop,
	}

	handlers := command.New(mgr, runtime, rmqClient, container, deviceMeta, logger)
	if err := handlers.Register(rootCtx); err != nil {
		return err
	}

	if err := mgr.AutoRestoreFromContainer(rootCtx, runtime); err != nil {
		logger.Warn("auto restore failed", slog.Any("err", err))
	}

	healthSrv := health.New(rmqClient, minioClient, mgr, cfg.HealthPort)
	healthErrCh := make(chan error, 1)
	go func() {
		err := healthSrv.ListenAndServe()
		healthErrCh <- err
	}()

	logger.Info("worker ready",
		slog.String("health_port", cfg.HealthPort),
		slog.Int("sessions", mgr.Count()),
	)

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-healthErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server crashed", slog.Any("err", err))
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer shutdownCancel()

	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("health shutdown failed", slog.Any("err", err))
	}
	if err := mgr.Shutdown(shutdownCtx); err != nil {
		logger.Warn("manager shutdown failed", slog.Any("err", err))
	}
	if err := rmqClient.Shutdown(shutdownCtx); err != nil {
		logger.Warn("rmq shutdown failed", slog.Any("err", err))
	}

	logger.Info("worker shutdown complete")
	return nil
}
