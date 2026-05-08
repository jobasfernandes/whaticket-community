package rmq

import (
	"encoding/json"
	"fmt"
	"time"

	"context"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	platerrors "github.com/canove/whaticket-community/backend/internal/platform/errors"
)

func (c *Client) Call(ctx context.Context, exchange, routingKey string, reqPayload any, respPayload any) error {
	start := time.Now()

	switch c.state.Load() {
	case stateDisabled:
		c.recordRPC(routingKey, outcomeDisabled, time.Since(start), ErrDisabled)
		return ErrDisabled
	case stateShuttingDown:
		c.recordRPC(routingKey, outcomeShuttingDown, time.Since(start), ErrShuttingDown)
		return ErrShuttingDown
	case stateConnected:
	default:
		c.recordRPC(routingKey, outcomeNotConnected, time.Since(start), ErrNotConnected)
		return ErrNotConnected
	}

	replyQueue := c.currentReplyQueue()
	if replyQueue == "" {
		c.recordRPC(routingKey, outcomeNotConnected, time.Since(start), ErrNotConnected)
		return ErrNotConnected
	}

	env, err := WrapPayload(routingKey, 0, reqPayload)
	if err != nil {
		c.recordRPC(routingKey, outcomeError, time.Since(start), err)
		return err
	}
	body, err := json.Marshal(env)
	if err != nil {
		c.recordRPC(routingKey, outcomeError, time.Since(start), err)
		return fmt.Errorf("rmq: marshal request envelope: %w", err)
	}

	correlationID := uuid.NewString()
	replyChan := make(chan replyMessage, 1)

	c.pendingMu.Lock()
	c.pending[correlationID] = replyChan
	c.recordInflightRPC(len(c.pending))
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, correlationID)
		c.recordInflightRPC(len(c.pending))
		c.pendingMu.Unlock()
	}()

	props := amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		CorrelationId: correlationID,
		ReplyTo:       replyQueue,
		Body:          body,
	}
	if err := c.publishBytes(ctx, exchange, routingKey, body, props); err != nil {
		c.recordRPC(routingKey, outcomeError, time.Since(start), err)
		return err
	}

	select {
	case msg := <-replyChan:
		if msg.err != nil {
			c.recordRPC(routingKey, outcomeError, time.Since(start), msg.err)
			return msg.err
		}
		var respEnv Envelope
		if err := json.Unmarshal(msg.body, &respEnv); err != nil {
			c.recordRPC(routingKey, outcomeError, time.Since(start), err)
			return fmt.Errorf("rmq: decode reply envelope: %w", err)
		}
		if respEnv.Error != nil {
			appErr := platerrors.New(respEnv.Error.Code, respEnv.Error.Status)
			c.recordRPC(routingKey, outcomeHandlerError, time.Since(start), appErr)
			return appErr
		}
		if respPayload != nil && len(respEnv.Payload) > 0 {
			if err := json.Unmarshal(respEnv.Payload, respPayload); err != nil {
				c.recordRPC(routingKey, outcomeError, time.Since(start), err)
				return fmt.Errorf("rmq: decode reply payload: %w", err)
			}
		}
		c.recordRPC(routingKey, outcomeOK, time.Since(start), nil)
		return nil
	case <-ctx.Done():
		c.recordRPC(routingKey, outcomeTimeout, time.Since(start), ctx.Err())
		return ctx.Err()
	}
}
