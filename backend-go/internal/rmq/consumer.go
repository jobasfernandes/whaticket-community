package rmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type HandlerFunc func(ctx context.Context, env Envelope) error

func (c *Client) Consume(ctx context.Context, queueName, consumerTag string, handler HandlerFunc) error {
	spec := &consumerSpec{
		queueName:   queueName,
		consumerTag: consumerTag,
		handler:     handler,
		ctx:         ctx,
	}

	c.runConsumer(spec)

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func (c *Client) runConsumer(spec *consumerSpec) {
	for {
		if spec.ctx.Err() != nil {
			return
		}
		if c.state.Load() == stateShuttingDown {
			return
		}
		if c.state.Load() == stateDisabled {
			return
		}

		conn := c.currentConn()
		if conn == nil || c.state.Load() != stateConnected {
			t := time.NewTimer(500 * time.Millisecond)
			select {
			case <-spec.ctx.Done():
				t.Stop()
				return
			case <-c.runCtx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			continue
		}

		ch, err := conn.Channel()
		if err != nil {
			c.logger().Warn("rmq consumer open channel failed",
				slog.String("queue", spec.queueName),
				slog.Any("err", err),
			)
			c.waitBeforeRetry(spec.ctx)
			continue
		}
		if err := ch.Qos(c.cfg.PrefetchCount, 0, false); err != nil {
			c.logger().Warn("rmq consumer qos failed",
				slog.String("queue", spec.queueName),
				slog.Any("err", err),
			)
			_ = ch.Close()
			c.waitBeforeRetry(spec.ctx)
			continue
		}
		deliveries, err := ch.Consume(spec.queueName, spec.consumerTag, false, false, false, false, nil)
		if err != nil {
			c.logger().Warn("rmq consumer subscribe failed",
				slog.String("queue", spec.queueName),
				slog.Any("err", err),
			)
			_ = ch.Close()
			c.waitBeforeRetry(spec.ctx)
			continue
		}

		c.dispatchLoop(spec, deliveries)
		_ = ch.Close()

		if spec.ctx.Err() != nil {
			return
		}
	}
}

func (c *Client) waitBeforeRetry(ctx context.Context) {
	t := time.NewTimer(c.cfg.ReconnectBackoff)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-c.runCtx.Done():
	case <-t.C:
	}
}

func (c *Client) dispatchLoop(spec *consumerSpec, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			c.handlerWG.Add(1)
			go c.dispatchDelivery(spec, d)
		case <-spec.ctx.Done():
			return
		case <-c.runCtx.Done():
			return
		}
	}
}

func (c *Client) dispatchDelivery(spec *consumerSpec, d amqp.Delivery) {
	defer c.handlerWG.Done()
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			c.logger().Error("rmq handler panic",
				slog.String("queue", spec.queueName),
				slog.String("routing_key", d.RoutingKey),
				slog.Any("panic", r),
				slog.String("stack", string(stack)),
			)
			_ = d.Nack(false, false)
			c.recordConsume(spec.queueName, d.RoutingKey, outcomePanic, time.Since(start), fmt.Errorf("panic: %v", r))
		}
	}()

	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		c.logger().Warn("rmq invalid envelope",
			slog.String("queue", spec.queueName),
			slog.String("routing_key", d.RoutingKey),
			slog.Any("err", err),
		)
		_ = d.Nack(false, false)
		c.recordConsume(spec.queueName, d.RoutingKey, outcomeInvalid, time.Since(start), err)
		return
	}

	if err := spec.handler(spec.ctx, env); err != nil {
		_ = d.Nack(false, true)
		c.recordConsume(spec.queueName, d.RoutingKey, outcomeNackRequeue, time.Since(start), err)
		return
	}

	if err := d.Ack(false); err != nil {
		c.logger().Warn("rmq ack failed",
			slog.String("queue", spec.queueName),
			slog.String("routing_key", d.RoutingKey),
			slog.Any("err", err),
		)
	}
	c.recordConsume(spec.queueName, d.RoutingKey, outcomeAck, time.Since(start), nil)
}
