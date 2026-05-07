package waevents

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/mustache"
)

const (
	queueDebounceInterval = 3 * time.Second
	rpcSendTimeout        = 10 * time.Second
	lrmPrefix             = "‎"
	whatsappUserSuffix    = "@s.whatsapp.net"
	groupSuffix           = "@g.us"
)

const rpcSendTextRoutingKey = "message.send.text"

type sendTextRequest struct {
	WhatsappID uint   `json:"whatsappId"`
	To         string `json:"to"`
	Body       string `json:"body"`
}

type sendTextResponse struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

type SendDebouncer struct {
	mu     sync.Mutex
	timers map[uint]*time.Timer
	rpc    RPCClient
	log    *slog.Logger
}

func NewSendDebouncer(rpc RPCClient, log *slog.Logger) *SendDebouncer {
	if log == nil {
		log = slog.Default()
	}
	return &SendDebouncer{
		timers: make(map[uint]*time.Timer),
		rpc:    rpc,
		log:    log,
	}
}

func (d *SendDebouncer) Schedule(ticketID uint, whatsappID uint, contactNumber, body string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if existing, ok := d.timers[ticketID]; ok {
		existing.Stop()
	}

	d.timers[ticketID] = time.AfterFunc(queueDebounceInterval, func() {
		d.mu.Lock()
		delete(d.timers, ticketID)
		d.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), rpcSendTimeout)
		defer cancel()
		if err := sendOutbound(ctx, d.rpc, whatsappID, contactNumber, body); err != nil {
			d.log.Warn("waevents debounced queue greeting send failed",
				slog.Uint64("whatsapp_id", uint64(whatsappID)),
				slog.Uint64("ticket_id", uint64(ticketID)),
				slog.Any("err", err),
			)
		}
	})
}

func (d *SendDebouncer) Cancel(ticketID uint) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[ticketID]; ok {
		t.Stop()
		delete(d.timers, ticketID)
	}
}

func sendOutbound(ctx context.Context, rpc RPCClient, whatsappID uint, contactNumber, body string) error {
	if rpc == nil {
		return fmt.Errorf("waevents: rpc client is nil")
	}
	req := sendTextRequest{
		WhatsappID: whatsappID,
		To:         normalizeUserJID(contactNumber),
		Body:       body,
	}
	var resp sendTextResponse
	return rpc.Call(ctx, waRPCExchange, rpcSendTextRoutingKey, req, &resp)
}

func normalizeUserJID(numberOrJID string) string {
	if strings.Contains(numberOrJID, "@") {
		return numberOrJID
	}
	return numberOrJID + whatsappUserSuffix
}

func (c *Consumer) handleQueueLogic(ctx context.Context, w Whatsapp, t Ticket, contact Contact, body string) error {
	queues := w.GetQueues()
	if len(queues) == 0 {
		return nil
	}

	if len(queues) == 1 {
		if err := c.TicketSvc.UpdateQueue(ctx, t.GetID(), queues[0].GetID()); err != nil {
			c.Log.Warn("waevents auto-assign single queue failed",
				slog.Uint64("ticket_id", uint64(t.GetID())),
				slog.Uint64("queue_id", uint64(queues[0].GetID())),
				slog.Any("err", err),
			)
			return err
		}
		return nil
	}

	choice, err := strconv.Atoi(strings.TrimSpace(body))
	if err == nil && choice >= 1 && choice <= len(queues) {
		selected := queues[choice-1]
		if uerr := c.TicketSvc.UpdateQueue(ctx, t.GetID(), selected.GetID()); uerr != nil {
			c.Log.Warn("waevents queue selection update failed",
				slog.Uint64("ticket_id", uint64(t.GetID())),
				slog.Uint64("queue_id", uint64(selected.GetID())),
				slog.Any("err", uerr),
			)
			return uerr
		}
		greeting := lrmPrefix + mustache.RenderOrTemplate(selected.GetGreetingMessage(), map[string]any{
			"name": contact.GetName(),
		})
		sendCtx, cancel := context.WithTimeout(ctx, rpcSendTimeout)
		defer cancel()
		if serr := sendOutbound(sendCtx, c.RPC, w.GetID(), contact.GetNumber(), greeting); serr != nil {
			c.Log.Warn("waevents queue selection greeting send failed",
				slog.Uint64("whatsapp_id", uint64(w.GetID())),
				slog.Uint64("ticket_id", uint64(t.GetID())),
				slog.Any("err", serr),
			)
		}
		return nil
	}

	options := buildQueueOptions(queues)
	rendered := mustache.RenderOrTemplate(w.GetGreetingMessage()+"\n"+options, map[string]any{
		"name": contact.GetName(),
	})
	c.debouncer.Schedule(t.GetID(), w.GetID(), contact.GetNumber(), lrmPrefix+rendered)
	return nil
}

func buildQueueOptions(queues []QueueLike) string {
	var b strings.Builder
	for i, q := range queues {
		fmt.Fprintf(&b, "*%d* - %s\n", i+1, q.GetName())
	}
	return b.String()
}
