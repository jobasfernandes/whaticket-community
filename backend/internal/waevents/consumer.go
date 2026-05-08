package waevents

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/rmq"
)

const (
	waEventsQueueName   = "backend.wa-events"
	waEventsConsumerTag = "waevents"
	waEventTypePrefix   = "wa.event."
	waCommandsExchange  = "wa.commands"
	waRPCExchange       = "wa.rpc"
	waEventsExchange    = "wa.events"
)

type Consumer struct {
	DB          *gorm.DB
	RMQ         RMQPublisher
	RPC         RPCClient
	WS          WSPublisher
	WhatsappSvc WhatsappService
	ContactSvc  ContactService
	TicketSvc   TicketService
	MessageSvc  MessageService
	Log         *slog.Logger
	debouncer   *SendDebouncer
}

func (c *Consumer) Start(ctx context.Context, rmqClient *rmq.Client) error {
	if c.Log == nil {
		c.Log = slog.Default()
	}
	c.debouncer = NewSendDebouncer(c.RPC, c.Log)
	return rmqClient.Consume(ctx, waEventsQueueName, waEventsConsumerTag, c.dispatch)
}

func (c *Consumer) dispatch(ctx context.Context, env rmq.Envelope) error {
	id, eventName, ok := parseEventType(env.Type)
	if !ok {
		c.Log.Debug("waevents invalid event type",
			slog.String("event_type", env.Type),
		)
		return nil
	}

	switch eventName {
	case "qr.code":
		return c.handleQRCode(ctx, id, env.Payload)
	case "qr.timeout":
		return c.handleQRTimeout(ctx, id, env.Payload)
	case "qr.success":
		return c.handleQRSuccess(ctx, id, env.Payload)
	case "pairphone.code":
		return c.handlePairphoneCode(ctx, id, env.Payload)
	case "PairSuccess":
		return c.handlePairSuccess(ctx, id, env.Payload)
	case "Connected", "PushNameSetting":
		return c.handleConnected(ctx, id, env.Payload)
	case "Disconnected":
		return c.handleDisconnected(ctx, id, env.Payload)
	case "LoggedOut":
		return c.handleLoggedOut(ctx, id, env.Payload)
	case "StreamReplaced":
		return c.handleStreamReplaced(ctx, id, env.Payload)
	case "TemporaryBan":
		return c.handleTemporaryBan(ctx, id, env.Payload)
	case "ConnectFailure":
		return c.handleConnectFailure(ctx, id, env.Payload)
	case "KeepAliveTimeout":
		return c.handleKeepAliveTimeout(ctx, id, env.Payload)
	case "KeepAliveRestored":
		return c.handleKeepAliveRestored(ctx, id, env.Payload)
	case "SessionRestored":
		return c.handleSessionRestored(ctx, id, env.Payload)
	case "SessionAlreadyRunning":
		c.Log.Info("waevents session already running",
			slog.Uint64("whatsapp_id", uint64(id)),
		)
		return nil
	case "Message":
		return c.handleMessage(ctx, id, env.Payload)
	case "Receipt":
		return c.handleReceipt(ctx, id, env.Payload)
	default:
		return c.handleNoOp(ctx, id, eventName, env.Payload)
	}
}

func parseEventType(eventType string) (uint, string, bool) {
	if !strings.HasPrefix(eventType, waEventTypePrefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(eventType, waEventTypePrefix)
	dot := strings.IndexByte(rest, '.')
	if dot <= 0 || dot == len(rest)-1 {
		return 0, "", false
	}
	idPart := rest[:dot]
	eventName := rest[dot+1:]
	idVal, err := strconv.ParseUint(idPart, 10, 64)
	if err != nil {
		return 0, "", false
	}
	return uint(idVal), eventName, true
}

func ticketChannel(ticketID uint) string {
	return fmt.Sprintf("ticket:%d", ticketID)
}
