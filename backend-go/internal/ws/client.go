package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	userID  uint
	profile string
	send    chan []byte
	closed  chan struct{}
	log     *slog.Logger

	closeOnce sync.Once
	teardown  sync.Once
}

type clientInbound struct {
	Action  string `json:"action"`
	Channel string `json:"channel"`
}

func newClient(hub *Hub, conn *websocket.Conn, userID uint, profile string, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{
		hub:     hub,
		conn:    conn,
		userID:  userID,
		profile: profile,
		send:    make(chan []byte, hub.cfg.ClientWriteBuffer),
		closed:  make(chan struct{}),
		log:     log,
	}
}

func (c *Client) readPump(ctx context.Context, ticketAuthz TicketAuthorizer) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("ws readPump panic recovered",
				"userId", c.userID,
				"recover", r,
			)
		}
		c.close()
	}()

	c.conn.SetReadLimit(maxInboundMessageBytes)

	for {
		var msg clientInbound
		err := wsjson.Read(ctx, c.conn, &msg)
		if err != nil {
			return
		}
		switch msg.Action {
		case "subscribe":
			result := authorizeSubscribe(ctx, ticketAuthz, c.userID, c.profile, msg.Channel)
			if !result.Allowed {
				c.log.Debug("ws subscribe denied",
					"userId", c.userID,
					"channel", msg.Channel,
					"code", result.Code,
				)
				c.sendError(msg.Channel, result.Code)
				continue
			}
			c.hub.subscribe(c, msg.Channel)
			c.log.Debug("ws subscribed",
				"userId", c.userID,
				"channel", msg.Channel,
			)
		case "unsubscribe":
			c.hub.unsubscribe(c, msg.Channel)
			c.log.Debug("ws unsubscribed",
				"userId", c.userID,
				"channel", msg.Channel,
			)
		default:
			c.log.Debug("ws unknown action",
				"userId", c.userID,
				"action", msg.Action,
			)
		}
	}
}

func (c *Client) writePump(ctx context.Context, pingInterval, writeTimeout, pongTimeout time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("ws writePump panic recovered",
				"userId", c.userID,
				"recover", r,
			)
		}
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closed:
			return
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(wctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			wctx, cancel := context.WithTimeout(ctx, pongTimeout)
			err := c.conn.Ping(wctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) sendError(channel, code string) {
	c.queueSystem(wireFrame{
		Channel: channel,
		Event:   EventError,
		Data:    map[string]string{"code": code},
	})
}

func (c *Client) sendSystem(event string, data any) {
	c.queueSystem(wireFrame{
		Channel: ChannelSystem,
		Event:   event,
		Data:    data,
	})
}

func (c *Client) queueSystem(frame wireFrame) {
	payload, err := json.Marshal(frame)
	if err != nil {
		c.log.Error("ws queueSystem marshal failed",
			"channel", frame.Channel,
			"event", frame.Event,
			"err", err,
		)
		return
	}
	select {
	case <-c.closed:
		return
	case c.send <- payload:
	default:
		c.hub.markSlowConsumer(c)
	}
}

func (c *Client) signalClose() {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
}

func (c *Client) close() {
	c.signalClose()
	c.teardown.Do(func() {
		c.hub.unregisterClient(c)
	})
}
