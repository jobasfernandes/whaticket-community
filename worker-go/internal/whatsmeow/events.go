package whatsmeow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/jobasfernandes/whaticket-go-worker/internal/media"
	"github.com/jobasfernandes/whaticket-go-worker/internal/rmq"
)

const (
	eventsExchange    = "wa.events"
	statusBroadcastID = "status@broadcast"
)

type MediaStore interface {
	Upload(ctx context.Context, objectKey string, data []byte, mimeType string) (string, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, env rmq.Envelope) error
}

type EventHandler struct {
	mgr   *Manager
	rmq   EventPublisher
	store MediaStore
	log   *slog.Logger
}

func NewEventHandler(mgr *Manager, publisher EventPublisher, store MediaStore, log *slog.Logger) *EventHandler {
	if log == nil {
		log = slog.Default()
	}
	return &EventHandler{
		mgr:   mgr,
		rmq:   publisher,
		store: store,
		log:   log,
	}
}

func (h *EventHandler) Handle(connID int) func(rawEvt any) {
	return func(rawEvt any) {
		defer func() {
			if r := recover(); r != nil {
				h.log.Error("event handler panic recovered",
					slog.Int("conn_id", connID),
					slog.String("event_type", fmt.Sprintf("%T", rawEvt)),
					slog.Any("panic", r),
				)
			}
		}()
		h.dispatch(connID, rawEvt)
	}
}

func (h *EventHandler) dispatch(connID int, rawEvt any) {
	ctx := context.Background()
	typeName := eventTypeName(rawEvt)

	if _, isQR := rawEvt.(*events.QR); isQR {
		return
	}

	switch evt := rawEvt.(type) {
	case *events.Message:
		payload, drop := h.processMessage(ctx, connID, evt)
		if drop {
			return
		}
		h.publish(ctx, connID, "Message", payload)

	case *events.Receipt:
		state := receiptStateLabel(evt.Type)
		h.publish(ctx, connID, typeName, map[string]any{
			"event": evt,
			"state": state,
		})

	case *events.LoggedOut:
		h.publish(ctx, connID, typeName, map[string]any{"event": evt})
		h.mgr.SendKill(connID)

	case *events.StreamReplaced:
		h.publish(ctx, connID, typeName, map[string]any{"event": evt})
		h.mgr.SendKill(connID)

	case *events.TemporaryBan:
		h.publish(ctx, connID, typeName, map[string]any{"event": evt})
		h.mgr.SendKill(connID)

	case *events.PushNameSetting:
		h.publish(ctx, connID, "Connected", map[string]any{"event": evt})

	case *events.AppStateSyncComplete:
		h.publish(ctx, connID, typeName, map[string]any{"event": evt})
		if evt.Name == appstate.WAPatchCriticalBlock {
			if sess, ok := h.mgr.Get(connID); ok && sess.Client != nil {
				if err := sess.Client.SendPresence(ctx, types.PresenceAvailable); err != nil {
					h.log.Warn("send presence after critical_block failed",
						slog.Int("conn_id", connID),
						slog.Any("err", err),
					)
				}
			}
		}

	case *events.HistorySync:
		h.publish(ctx, connID, typeName, buildHistorySyncPayload(evt))

	case *events.PairSuccess,
		*events.Connected,
		*events.Disconnected,
		*events.ConnectFailure,
		*events.KeepAliveTimeout,
		*events.KeepAliveRestored,
		*events.GroupInfo,
		*events.JoinedGroup,
		*events.Picture,
		*events.Presence,
		*events.ChatPresence,
		*events.CallOffer,
		*events.CallAccept,
		*events.CallTerminate,
		*events.PrivacySettings,
		*events.OfflineSyncCompleted,
		*events.OfflineSyncPreview,
		*events.NewsletterJoin,
		*events.NewsletterLeave,
		*events.NewsletterMuteChange,
		*events.NewsletterLiveUpdate:
		h.publish(ctx, connID, typeName, map[string]any{"event": evt})

	default:
		h.log.Debug("unknown event",
			slog.Int("conn_id", connID),
			slog.String("event_type", fmt.Sprintf("%T", rawEvt)),
		)
	}
}

func (h *EventHandler) processMessage(ctx context.Context, connID int, evt *events.Message) (any, bool) {
	body := extractBody(evt.Message)
	if HasLRMPrefix(body) {
		h.log.Info("dropped lrm",
			slog.Int("conn_id", connID),
			slog.String("msg_id", evt.Info.ID),
		)
		return nil, true
	}

	sess, ok := h.mgr.Get(connID)
	if ok {
		if sess.Settings.IgnoreGroups && evt.Info.Chat.Server == types.GroupServer {
			return nil, true
		}
		if sess.Settings.IgnoreStatus && evt.Info.Chat.String() == statusBroadcastID {
			return nil, true
		}
	}

	if !ok || sess.Client == nil {
		return media.IncomingPayload{Event: evt}, false
	}

	payload, err := media.ProcessIncoming(ctx, sess.Client, connID, evt, h.store, sess.MediaMode, h.log)
	if err != nil {
		h.log.Error("media process failed",
			slog.Int("conn_id", connID),
			slog.String("msg_id", evt.Info.ID),
			slog.Any("err", err),
		)
		payload = media.IncomingPayload{Event: evt}
	}

	if sess.Settings.ReadMessages && !evt.Info.IsFromMe {
		ScheduleMarkRead(sess.Client, evt.Info.ID, evt.Info.Timestamp, evt.Info.Chat, evt.Info.Sender, h.log)
	}

	return payload, false
}

func (h *EventHandler) publish(ctx context.Context, connID int, eventType string, payload any) {
	if h.rmq == nil {
		return
	}
	routingKey := fmt.Sprintf("wa.event.%d.%s", connID, eventType)
	env, err := rmq.WrapPayload(routingKey, connID, payload)
	if err != nil {
		h.log.Error("wrap payload failed",
			slog.Int("conn_id", connID),
			slog.String("event_type", eventType),
			slog.Any("err", err),
		)
		return
	}
	if err := h.rmq.Publish(ctx, eventsExchange, routingKey, env); err != nil {
		h.log.Error("publish event failed",
			slog.Int("conn_id", connID),
			slog.String("event_type", eventType),
			slog.String("routing_key", routingKey),
			slog.Any("err", err),
		)
	}
}

func eventTypeName(evt any) string {
	name := fmt.Sprintf("%T", evt)
	return strings.TrimPrefix(name, "*events.")
}

func receiptStateLabel(rt types.ReceiptType) string {
	switch rt {
	case types.ReceiptTypeRead:
		return "Read"
	case types.ReceiptTypeReadSelf:
		return "ReadSelf"
	case types.ReceiptTypeDelivered:
		return "Delivered"
	default:
		return string(rt)
	}
}

func buildHistorySyncPayload(evt *events.HistorySync) map[string]any {
	payload := map[string]any{
		"syncType":      "",
		"progress":      float32(0),
		"conversations": 0,
		"totalMessages": 0,
		"event":         evt,
	}
	if evt == nil || evt.Data == nil {
		return payload
	}
	payload["syncType"] = evt.Data.GetSyncType().String()
	if p := evt.Data.GetProgress(); p > 0 {
		payload["progress"] = float32(p) / 100
	}
	conversations := evt.Data.GetConversations()
	payload["conversations"] = len(conversations)
	payload["totalMessages"] = countHistoryMessages(conversations)
	return payload
}

func countHistoryMessages(conversations []*waHistorySync.Conversation) int {
	total := 0
	for _, c := range conversations {
		if c == nil {
			continue
		}
		total += len(c.GetMessages())
	}
	return total
}

func extractBody(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if conv := msg.GetConversation(); conv != "" {
		return conv
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption()
	}
	if video := msg.GetVideoMessage(); video != nil {
		return video.GetCaption()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetCaption()
	}
	return ""
}
