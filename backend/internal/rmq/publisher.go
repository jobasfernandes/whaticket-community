package rmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const messageSizeWarnThreshold = 128 * 1024

func (c *Client) Publish(ctx context.Context, exchange, routingKey string, env Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("rmq: marshal envelope: %w", err)
	}
	return c.publishBytes(ctx, exchange, routingKey, body, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (c *Client) publishBytes(ctx context.Context, exchange, routingKey string, body []byte, props amqp.Publishing) error {
	start := time.Now()

	switch c.state.Load() {
	case stateDisabled:
		c.recordPublish(exchange, routingKey, outcomeDisabled, time.Since(start), ErrDisabled)
		return ErrDisabled
	case stateShuttingDown:
		c.recordPublish(exchange, routingKey, outcomeShuttingDown, time.Since(start), ErrShuttingDown)
		return ErrShuttingDown
	case stateConnected:
	default:
		c.recordPublish(exchange, routingKey, outcomeNotConnected, time.Since(start), ErrNotConnected)
		return ErrNotConnected
	}

	if len(body) > messageSizeWarnThreshold {
		c.logger().Warn("rmq large message",
			slog.String("exchange", exchange),
			slog.String("routing_key", routingKey),
			slog.Int("size_bytes", len(body)),
		)
	}

	if props.Body == nil {
		props.Body = body
	}
	if props.ContentType == "" {
		props.ContentType = "application/json"
	}
	if props.DeliveryMode == 0 {
		props.DeliveryMode = amqp.Persistent
	}

	c.publishMu.Lock()
	defer c.publishMu.Unlock()

	if c.state.Load() != stateConnected {
		c.recordPublish(exchange, routingKey, outcomeNotConnected, time.Since(start), ErrNotConnected)
		return ErrNotConnected
	}
	if c.publishCh == nil || c.confirms == nil {
		c.recordPublish(exchange, routingKey, outcomeNotConnected, time.Since(start), ErrNotConnected)
		return ErrNotConnected
	}

	if err := c.publishCh.PublishWithContext(ctx, exchange, routingKey, false, false, props); err != nil {
		c.recordPublish(exchange, routingKey, outcomeError, time.Since(start), err)
		return fmt.Errorf("rmq: publish: %w", err)
	}

	timer := time.NewTimer(c.cfg.PublishConfirmTimeout)
	defer timer.Stop()

	select {
	case confirm, ok := <-c.confirms:
		if !ok {
			c.recordPublish(exchange, routingKey, outcomeError, time.Since(start), ErrConnectionLost)
			return ErrConnectionLost
		}
		if !confirm.Ack {
			c.recordPublish(exchange, routingKey, outcomeError, time.Since(start), ErrPublishConfirmTimeout)
			return fmt.Errorf("rmq: publish nacked: %w", ErrPublishConfirmTimeout)
		}
		c.recordPublish(exchange, routingKey, outcomeOK, time.Since(start), nil)
		return nil
	case <-timer.C:
		c.recordPublish(exchange, routingKey, outcomeTimeout, time.Since(start), ErrPublishConfirmTimeout)
		return ErrPublishConfirmTimeout
	case <-ctx.Done():
		c.recordPublish(exchange, routingKey, outcomeError, time.Since(start), ctx.Err())
		return ctx.Err()
	}
}
