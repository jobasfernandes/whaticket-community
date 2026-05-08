//go:build integration

package rmqtest

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	"github.com/canove/whaticket-community/backend/internal/rmq"
)

const (
	rabbitmqImage  = "rabbitmq:3.13-management-alpine"
	defaultTimeout = 90 * time.Second
)

type RabbitMQ struct {
	Container *rabbitmq.RabbitMQContainer
	URL       string
	Client    *rmq.Client
}

func StartRabbitMQ(ctx context.Context, t *testing.T) *RabbitMQ {
	return startRabbitMQWithRole(ctx, t, rmq.RoleBackend)
}

func StartRabbitMQAsWorker(ctx context.Context, t *testing.T) *RabbitMQ {
	return startRabbitMQWithRole(ctx, t, rmq.RoleWorker)
}

func startRabbitMQWithRole(ctx context.Context, t *testing.T, role rmq.Role) *RabbitMQ {
	t.Helper()

	startCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	container, err := rabbitmq.Run(startCtx, rabbitmqImage)
	if err != nil {
		t.Skipf("rabbitmq testcontainer unavailable: %v", err)
		return nil
	}

	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	url, err := container.AmqpURL(startCtx)
	if err != nil {
		t.Fatalf("rabbitmq amqp url: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := rmq.New(rmq.Config{
		URL:                   url,
		ReconnectAttempts:     3,
		ReconnectBackoff:      500 * time.Millisecond,
		PublishConfirmTimeout: 5 * time.Second,
		Logger:                logger,
	})
	client.SetRole(role)
	if err := client.Start(startCtx); err != nil {
		t.Fatalf("rmq client start: %v", err)
	}

	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = client.Shutdown(shutCtx)
	})

	return &RabbitMQ{
		Container: container,
		URL:       url,
		Client:    client,
	}
}
