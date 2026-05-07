package rmq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	stateDisconnected int32 = 0
	stateConnecting   int32 = 1
	stateConnected    int32 = 2
	stateDisabled     int32 = 3
	stateShuttingDown int32 = 4
)

const (
	defaultReconnectAttempts     = 10
	defaultReconnectBackoff      = 3 * time.Second
	defaultPublishConfirmTimeout = 5 * time.Second
	defaultPrefetchCount         = 16
)

type Config struct {
	URL                   string
	ReconnectAttempts     int
	ReconnectBackoff      time.Duration
	PublishConfirmTimeout time.Duration
	PrefetchCount         int
	Logger                *slog.Logger
}

type Role int

const (
	RoleBackend Role = iota
	RoleWorker
)

type consumerSpec struct {
	queueName   string
	consumerTag string
	handler     HandlerFunc
	ctx         context.Context
}

type replyMessage struct {
	body []byte
	err  error
}

type Client struct {
	cfg  Config
	role Role
	log  *slog.Logger

	state atomic.Int32

	connMu sync.RWMutex
	conn   *amqp.Connection

	publishMu sync.Mutex
	publishCh *amqp.Channel
	confirms  chan amqp.Confirmation

	replyMu        sync.RWMutex
	replyCh        *amqp.Channel
	replyQueueName string

	pendingMu sync.RWMutex
	pending   map[string]chan replyMessage

	handlerWG sync.WaitGroup

	runCtx    context.Context
	runCancel context.CancelFunc

	reconnectWG sync.WaitGroup
}

func New(cfg Config) *Client {
	if cfg.ReconnectAttempts <= 0 {
		cfg.ReconnectAttempts = defaultReconnectAttempts
	}
	if cfg.ReconnectBackoff <= 0 {
		cfg.ReconnectBackoff = defaultReconnectBackoff
	}
	if cfg.PublishConfirmTimeout <= 0 {
		cfg.PublishConfirmTimeout = defaultPublishConfirmTimeout
	}
	if cfg.PrefetchCount <= 0 {
		cfg.PrefetchCount = defaultPrefetchCount
	}
	c := &Client{
		cfg:     cfg,
		role:    RoleBackend,
		log:     cfg.Logger,
		pending: make(map[string]chan replyMessage),
	}
	c.state.Store(stateDisconnected)
	return c
}

func (c *Client) SetRole(role Role) {
	c.role = role
}

func (c *Client) IsConnected() bool {
	return c.state.Load() == stateConnected
}

func (c *Client) Disabled() bool {
	return c.state.Load() == stateDisabled
}

func (c *Client) Start(ctx context.Context) error {
	if !c.state.CompareAndSwap(stateDisconnected, stateConnecting) {
		return fmt.Errorf("rmq: Start called in invalid state: %s", stateName(c.state.Load()))
	}
	c.recordStateTransition(stateDisconnected, stateConnecting)

	c.runCtx, c.runCancel = context.WithCancel(context.Background())

	if err := c.dialAndSetup(ctx); err != nil {
		c.transitionTo(stateDisabled)
		c.recordReconnectExhausted(c.cfg.ReconnectAttempts)
		return err
	}

	c.transitionTo(stateConnected)

	c.reconnectWG.Add(1)
	go c.reconnectLoop()

	return nil
}

func (c *Client) Shutdown(ctx context.Context) error {
	prev := c.state.Swap(stateShuttingDown)
	c.recordStateTransition(prev, stateShuttingDown)

	if c.runCancel != nil {
		c.runCancel()
	}

	c.failPendingRPCs(ErrShuttingDown)

	done := make(chan struct{})
	go func() {
		c.handlerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	c.connMu.Lock()
	if c.publishCh != nil {
		_ = c.publishCh.Close()
		c.publishCh = nil
	}
	if c.replyCh != nil {
		_ = c.replyCh.Close()
		c.replyCh = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	doneRecon := make(chan struct{})
	go func() {
		c.reconnectWG.Wait()
		close(doneRecon)
	}()
	select {
	case <-doneRecon:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (c *Client) transitionTo(to int32) {
	prev := c.state.Swap(to)
	if prev != to {
		c.recordStateTransition(prev, to)
	}
}

func (c *Client) dialAndSetup(ctx context.Context) error {
	var lastErr error
	for attempt := 1; attempt <= c.cfg.ReconnectAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := amqp.Dial(c.cfg.URL)
		if err == nil {
			c.connMu.Lock()
			c.conn = conn
			c.connMu.Unlock()

			if setupErr := c.setupChannels(); setupErr != nil {
				_ = conn.Close()
				c.connMu.Lock()
				c.conn = nil
				c.connMu.Unlock()
				lastErr = setupErr
			} else {
				return nil
			}
		} else {
			lastErr = err
		}
		c.recordReconnectAttempt(attempt, c.cfg.ReconnectBackoff, lastErr)
		if attempt == c.cfg.ReconnectAttempts {
			break
		}
		t := time.NewTimer(c.cfg.ReconnectBackoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("rmq: dial failed after %d attempts", c.cfg.ReconnectAttempts)
	}
	return lastErr
}

func (c *Client) setupChannels() error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return ErrNotConnected
	}

	pubCh, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("rmq: open publish channel: %w", err)
	}
	if err := pubCh.Confirm(false); err != nil {
		_ = pubCh.Close()
		return fmt.Errorf("rmq: enable publisher confirms: %w", err)
	}
	confirms := pubCh.NotifyPublish(make(chan amqp.Confirmation, 1))

	if err := declareCommonExchanges(pubCh); err != nil {
		_ = pubCh.Close()
		return err
	}

	switch c.role {
	case RoleBackend:
		if err := declareBackendQueues(pubCh); err != nil {
			_ = pubCh.Close()
			return err
		}
	case RoleWorker:
		if err := declareWorkerQueues(pubCh); err != nil {
			_ = pubCh.Close()
			return err
		}
	}

	replyCh, err := conn.Channel()
	if err != nil {
		_ = pubCh.Close()
		return fmt.Errorf("rmq: open reply channel: %w", err)
	}
	replyName, err := declareReplyQueue(replyCh)
	if err != nil {
		_ = pubCh.Close()
		_ = replyCh.Close()
		return err
	}
	deliveries, err := replyCh.Consume(replyName, "rpc-reply-"+replyName, true, true, false, false, nil)
	if err != nil {
		_ = pubCh.Close()
		_ = replyCh.Close()
		return fmt.Errorf("rmq: subscribe reply queue: %w", err)
	}

	c.publishMu.Lock()
	c.publishCh = pubCh
	c.confirms = confirms
	c.publishMu.Unlock()

	c.replyMu.Lock()
	c.replyCh = replyCh
	c.replyQueueName = replyName
	c.replyMu.Unlock()

	go c.consumeReplies(deliveries)

	return nil
}

func (c *Client) reconnectLoop() {
	defer c.reconnectWG.Done()
	for {
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()
		if conn == nil {
			return
		}
		closeChan := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-c.runCtx.Done():
			return
		case amqpErr, ok := <-closeChan:
			if c.state.Load() == stateShuttingDown {
				return
			}
			if !ok && amqpErr == nil {
				return
			}
			c.logger().Warn("rmq connection closed", slog.Any("err", amqpErr))
			c.transitionTo(stateDisconnected)
			c.failPendingRPCs(ErrConnectionLost)
			c.publishMu.Lock()
			c.publishCh = nil
			c.confirms = nil
			c.publishMu.Unlock()
			c.replyMu.Lock()
			c.replyCh = nil
			c.replyQueueName = ""
			c.replyMu.Unlock()
			c.connMu.Lock()
			c.conn = nil
			c.connMu.Unlock()

			c.transitionTo(stateConnecting)
			if err := c.dialAndSetup(c.runCtx); err != nil {
				c.transitionTo(stateDisabled)
				c.recordReconnectExhausted(c.cfg.ReconnectAttempts)
				return
			}
			c.transitionTo(stateConnected)
		}
	}
}

func (c *Client) consumeReplies(deliveries <-chan amqp.Delivery) {
	for d := range deliveries {
		corr := d.CorrelationId
		c.pendingMu.RLock()
		ch, ok := c.pending[corr]
		c.pendingMu.RUnlock()
		if !ok {
			c.logger().Warn("rmq late or unknown rpc reply",
				slog.String("correlation_id", corr),
			)
			continue
		}
		body := make([]byte, len(d.Body))
		copy(body, d.Body)
		select {
		case ch <- replyMessage{body: body}:
		default:
			c.logger().Warn("rmq dropped rpc reply (full chan)",
				slog.String("correlation_id", corr),
			)
		}
	}
}

func (c *Client) failPendingRPCs(err error) {
	c.pendingMu.Lock()
	for corr, ch := range c.pending {
		select {
		case ch <- replyMessage{err: err}:
		default:
		}
		delete(c.pending, corr)
	}
	c.pendingMu.Unlock()
}

func (c *Client) currentConn() *amqp.Connection {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.conn
}

func (c *Client) currentReplyQueue() string {
	c.replyMu.RLock()
	defer c.replyMu.RUnlock()
	return c.replyQueueName
}
