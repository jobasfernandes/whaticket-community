package rmq

import (
	"log/slog"
	"time"
)

const (
	outcomeOK           = "ok"
	outcomeError        = "error"
	outcomeDisabled     = "disabled"
	outcomeNotConnected = "not_connected"
	outcomeShuttingDown = "shutting_down"
	outcomeTimeout      = "timeout"
	outcomeAck          = "ack"
	outcomeNackRequeue  = "nack_requeue"
	outcomeNackDrop     = "nack_drop"
	outcomePanic        = "panic"
	outcomeHandlerError = "handler_error"
	outcomeUnknown      = "unknown_rpc"
	outcomeInvalid      = "invalid_envelope"
)

func (c *Client) logger() *slog.Logger {
	if c.log != nil {
		return c.log
	}
	return slog.Default()
}

func (c *Client) recordStateTransition(from, to int32) {
	c.logger().Info("rmq state transition",
		slog.String("from", stateName(from)),
		slog.String("to", stateName(to)),
	)
}

func (c *Client) recordReconnectAttempt(attempt int, delay time.Duration, err error) {
	level := slog.LevelWarn
	c.logger().Log(nil, level, "rmq reconnect attempt",
		slog.Int("attempt", attempt),
		slog.Duration("delay", delay),
		slog.Any("err", err),
	)
}

func (c *Client) recordReconnectExhausted(attempts int) {
	c.logger().Error("rmq reconnect exhausted",
		slog.Int("attempts", attempts),
	)
}

func (c *Client) recordPublish(exchange, routingKey, outcome string, duration time.Duration, err error) {
	level := slog.LevelDebug
	if outcome != outcomeOK {
		level = slog.LevelWarn
	}
	c.logger().Log(nil, level, "rmq publish",
		slog.String("exchange", exchange),
		slog.String("routing_key", routingKey),
		slog.String("outcome", outcome),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Any("err", err),
	)
}

func (c *Client) recordConsume(queue, routingKey, outcome string, duration time.Duration, err error) {
	level := slog.LevelDebug
	switch outcome {
	case outcomeNackRequeue, outcomeNackDrop, outcomeInvalid:
		level = slog.LevelWarn
	case outcomePanic:
		level = slog.LevelError
	}
	c.logger().Log(nil, level, "rmq consume",
		slog.String("queue", queue),
		slog.String("routing_key", routingKey),
		slog.String("outcome", outcome),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Any("err", err),
	)
}

func (c *Client) recordRPC(routingKey, outcome string, duration time.Duration, err error) {
	level := slog.LevelDebug
	if outcome != outcomeOK {
		level = slog.LevelWarn
	}
	c.logger().Log(nil, level, "rmq rpc call",
		slog.String("routing_key", routingKey),
		slog.String("outcome", outcome),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Any("err", err),
	)
}

func (c *Client) recordInflightRPC(size int) {
	c.logger().Debug("rmq inflight rpc", slog.Int("size", size))
}

func stateName(s int32) string {
	switch s {
	case stateDisconnected:
		return "disconnected"
	case stateConnecting:
		return "connecting"
	case stateConnected:
		return "connected"
	case stateDisabled:
		return "disabled"
	case stateShuttingDown:
		return "shutting_down"
	default:
		return "unknown"
	}
}
