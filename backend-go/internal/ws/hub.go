package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

type Hub struct {
	cfg Config
	log *slog.Logger

	mu       sync.RWMutex
	channels map[string]map[*Client]struct{}
	clients  map[*Client]map[string]struct{}

	shuttingDown atomic.Bool
}

type wireFrame struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
	Data    any    `json:"data"`
}

func NewHub(cfg Config) *Hub {
	cfg.applyDefaults()
	return &Hub{
		cfg:      cfg,
		log:      cfg.Logger,
		channels: make(map[string]map[*Client]struct{}),
		clients:  make(map[*Client]map[string]struct{}),
	}
}

func (h *Hub) registerClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.clients[c]; !exists {
		h.clients[c] = make(map[string]struct{})
	}
}

func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.clients[c]
	if !ok {
		return
	}
	for channel := range subs {
		if members, found := h.channels[channel]; found {
			delete(members, c)
			if len(members) == 0 {
				delete(h.channels, channel)
			}
		}
	}
	delete(h.clients, c)
}

func (h *Hub) subscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.clients[c]
	if !ok {
		return
	}
	if _, already := subs[channel]; already {
		return
	}
	subs[channel] = struct{}{}
	members, found := h.channels[channel]
	if !found {
		members = make(map[*Client]struct{})
		h.channels[channel] = members
	}
	members[c] = struct{}{}
	if len(subs) > 100 {
		h.log.Warn("ws client exceeds 100 subscriptions",
			"userId", c.userID,
			"profile", c.profile,
			"subscriptions", len(subs),
		)
	}
}

func (h *Hub) unsubscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.clients[c]
	if !ok {
		return
	}
	if _, has := subs[channel]; !has {
		return
	}
	delete(subs, channel)
	if members, found := h.channels[channel]; found {
		delete(members, c)
		if len(members) == 0 {
			delete(h.channels, channel)
		}
	}
}

func (h *Hub) Stats() Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, subs := range h.clients {
		total += len(subs)
	}
	return Stats{
		ActiveConnections:  len(h.clients),
		ActiveChannels:     len(h.channels),
		TotalSubscriptions: total,
	}
}

func (h *Hub) Publish(channel, event string, data any) {
	if h.shuttingDown.Load() {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("ws publish panic recovered",
				"channel", channel,
				"event", event,
				"recover", r,
			)
		}
	}()

	payload, err := json.Marshal(wireFrame{Channel: channel, Event: event, Data: data})
	if err != nil {
		h.log.Error("ws publish marshal failed",
			"channel", channel,
			"event", event,
			"err", err,
		)
		return
	}

	h.mu.RLock()
	members, ok := h.channels[channel]
	if !ok || len(members) == 0 {
		h.mu.RUnlock()
		h.log.Debug("ws publish without subscribers", "channel", channel, "event", event)
		return
	}
	targets := make([]*Client, 0, len(members))
	for c := range members {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case <-c.closed:
			continue
		case c.send <- payload:
		default:
			h.markSlowConsumer(c)
		}
	}
}

func (h *Hub) markSlowConsumer(c *Client) {
	h.log.Warn("ws slow consumer marked for disconnect",
		"userId", c.userID,
		"profile", c.profile,
	)
	c.signalClose()
}

func (h *Hub) Shutdown(ctx context.Context) error {
	h.shuttingDown.Store(true)

	h.mu.RLock()
	snapshot := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	closingPayload, err := json.Marshal(wireFrame{
		Channel: ChannelSystem,
		Event:   EventClosing,
		Data:    map[string]any{},
	})
	if err == nil {
		for _, c := range snapshot {
			select {
			case <-c.closed:
			case c.send <- closingPayload:
			default:
			}
		}
	}

	ticker := time.NewTicker(h.cfg.ShutdownPollEvery)
	defer ticker.Stop()

	for {
		if h.Stats().ActiveConnections == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			h.forceCloseRemaining()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (h *Hub) forceCloseRemaining() {
	h.mu.RLock()
	remaining := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		remaining = append(remaining, c)
	}
	h.mu.RUnlock()

	for _, c := range remaining {
		c.signalClose()
		_ = c.conn.Close(websocket.StatusGoingAway, "shutdown")
	}
}
