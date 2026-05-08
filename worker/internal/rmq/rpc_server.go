package rmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	platerrors "github.com/canove/whaticket-community/worker/internal/platform/errors"
)

type RPCHandlerFunc func(ctx context.Context, env Envelope) (any, error)

const (
	rpcUnknownCode  = "ERR_UNKNOWN_RPC"
	rpcInternalCode = "ERR_INTERNAL"
	rpcInvalidCode  = "ERR_INVALID_ENVELOPE"
)

func (c *Client) ServeRPC(ctx context.Context, queueName string, handlers map[string]RPCHandlerFunc) error {
	c.runRPCServer(ctx, queueName, handlers)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func (c *Client) runRPCServer(ctx context.Context, queueName string, handlers map[string]RPCHandlerFunc) {
	for {
		if ctx.Err() != nil {
			return
		}
		if c.state.Load() == stateShuttingDown || c.state.Load() == stateDisabled {
			return
		}

		conn := c.currentConn()
		if conn == nil || c.state.Load() != stateConnected {
			t := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
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
			c.logger().Warn("rmq rpc server open channel failed",
				slog.String("queue", queueName),
				slog.Any("err", err),
			)
			c.waitBeforeRetry(ctx)
			continue
		}
		if err := ch.Qos(c.cfg.PrefetchCount, 0, false); err != nil {
			_ = ch.Close()
			c.waitBeforeRetry(ctx)
			continue
		}
		deliveries, err := ch.Consume(queueName, "rpc-server-"+queueName, false, false, false, false, nil)
		if err != nil {
			c.logger().Warn("rmq rpc server subscribe failed",
				slog.String("queue", queueName),
				slog.Any("err", err),
			)
			_ = ch.Close()
			c.waitBeforeRetry(ctx)
			continue
		}

		c.dispatchRPCLoop(ctx, queueName, handlers, deliveries)
		_ = ch.Close()

		if ctx.Err() != nil {
			return
		}
	}
}

func (c *Client) dispatchRPCLoop(ctx context.Context, queueName string, handlers map[string]RPCHandlerFunc, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			c.handlerWG.Add(1)
			go c.dispatchRPC(ctx, queueName, handlers, d)
		case <-ctx.Done():
			return
		case <-c.runCtx.Done():
			return
		}
	}
}

func (c *Client) dispatchRPC(ctx context.Context, queueName string, handlers map[string]RPCHandlerFunc, d amqp.Delivery) {
	defer c.handlerWG.Done()
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			c.logger().Error("rmq rpc handler panic",
				slog.String("queue", queueName),
				slog.String("routing_key", d.RoutingKey),
				slog.Any("panic", r),
				slog.String("stack", string(stack)),
			)
			c.publishRPCError(ctx, d, rpcInternalCode, http.StatusInternalServerError, fmt.Sprintf("panic: %v", r))
			_ = d.Nack(false, false)
			c.recordConsume(queueName, d.RoutingKey, outcomePanic, time.Since(start), fmt.Errorf("panic: %v", r))
		}
	}()

	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		c.publishRPCError(ctx, d, rpcInvalidCode, http.StatusBadRequest, err.Error())
		_ = d.Nack(false, false)
		c.recordConsume(queueName, d.RoutingKey, outcomeInvalid, time.Since(start), err)
		return
	}

	handler, ok := handlers[env.Type]
	if !ok {
		c.publishRPCError(ctx, d, rpcUnknownCode, http.StatusNotFound, "unknown rpc type: "+env.Type)
		_ = d.Ack(false)
		c.recordConsume(queueName, d.RoutingKey, outcomeUnknown, time.Since(start), nil)
		return
	}

	resp, err := handler(ctx, env)
	if err != nil {
		var appErr *platerrors.AppError
		if errors.As(err, &appErr) {
			c.publishRPCError(ctx, d, appErr.Code, appErr.Status, appErr.Error())
		} else {
			c.publishRPCError(ctx, d, rpcInternalCode, http.StatusInternalServerError, err.Error())
		}
		_ = d.Ack(false)
		c.recordConsume(queueName, d.RoutingKey, outcomeHandlerError, time.Since(start), err)
		return
	}

	if err := c.publishRPCResponse(ctx, d, env.Type, env.UserID, resp); err != nil {
		c.logger().Warn("rmq rpc reply publish failed",
			slog.String("queue", queueName),
			slog.String("routing_key", d.RoutingKey),
			slog.Any("err", err),
		)
		_ = d.Nack(false, true)
		c.recordConsume(queueName, d.RoutingKey, outcomeNackRequeue, time.Since(start), err)
		return
	}

	if err := d.Ack(false); err != nil {
		c.logger().Warn("rmq rpc ack failed",
			slog.String("queue", queueName),
			slog.String("routing_key", d.RoutingKey),
			slog.Any("err", err),
		)
	}
	c.recordConsume(queueName, d.RoutingKey, outcomeAck, time.Since(start), nil)
}

func (c *Client) publishRPCResponse(ctx context.Context, d amqp.Delivery, eventType string, userID int, resp any) error {
	respEnv, err := WrapPayload(eventType, userID, resp)
	if err != nil {
		return err
	}
	body, err := json.Marshal(respEnv)
	if err != nil {
		return fmt.Errorf("rmq: marshal response envelope: %w", err)
	}
	props := amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		CorrelationId: d.CorrelationId,
		Body:          body,
	}
	return c.publishBytes(ctx, "", d.ReplyTo, body, props)
}

func (c *Client) publishRPCError(ctx context.Context, d amqp.Delivery, code string, status int, message string) {
	if d.ReplyTo == "" {
		return
	}
	respEnv := Envelope{
		Version:   envelopeVersion,
		Timestamp: time.Now().UnixMilli(),
		Type:      d.RoutingKey,
		Payload:   json.RawMessage("null"),
		Error: &EnvelopeError{
			Code:    code,
			Status:  status,
			Message: message,
		},
	}
	body, err := json.Marshal(respEnv)
	if err != nil {
		c.logger().Warn("rmq rpc error marshal failed",
			slog.String("routing_key", d.RoutingKey),
			slog.Any("err", err),
		)
		return
	}
	props := amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		CorrelationId: d.CorrelationId,
		Body:          body,
	}
	if err := c.publishBytes(ctx, "", d.ReplyTo, body, props); err != nil {
		c.logger().Warn("rmq rpc error publish failed",
			slog.String("routing_key", d.RoutingKey),
			slog.Any("err", err),
		)
	}
}
