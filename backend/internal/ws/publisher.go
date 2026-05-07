package ws

import (
	"context"
	"log/slog"
	"time"
)

type Publisher interface {
	Publish(channel, event string, data any)
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(_, _ string, _ any) {}

type TicketAuthorizer interface {
	CanSee(ctx context.Context, userID uint, profile string, ticketID uint) (bool, error)
}

type NoopTicketAuthorizer struct{}

func (NoopTicketAuthorizer) CanSee(_ context.Context, _ uint, _ string, _ uint) (bool, error) {
	return false, nil
}

type Config struct {
	JWTSecret         []byte
	PingInterval      time.Duration
	PongTimeout       time.Duration
	WriteTimeout      time.Duration
	ClientWriteBuffer int
	AllowedOrigin     string
	TicketAuthz       TicketAuthorizer
	Logger            *slog.Logger
	ShutdownPollEvery time.Duration
}

type Stats struct {
	ActiveConnections  int
	ActiveChannels     int
	TotalSubscriptions int
}

const (
	defaultPingInterval      = 30 * time.Second
	defaultPongTimeout       = 60 * time.Second
	defaultWriteTimeout      = 10 * time.Second
	defaultClientWriteBuffer = 64
	defaultShutdownPollEvery = 100 * time.Millisecond
	maxInboundMessageBytes   = 64 * 1024
)

func (c *Config) applyDefaults() {
	if c.PingInterval <= 0 {
		c.PingInterval = defaultPingInterval
	}
	if c.PongTimeout <= 0 {
		c.PongTimeout = defaultPongTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = defaultWriteTimeout
	}
	if c.ClientWriteBuffer <= 0 {
		c.ClientWriteBuffer = defaultClientWriteBuffer
	}
	if c.ShutdownPollEvery <= 0 {
		c.ShutdownPollEvery = defaultShutdownPollEvery
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.TicketAuthz == nil {
		c.TicketAuthz = NoopTicketAuthorizer{}
	}
}
