package whatsmeow

import (
	"context"
	"log/slog"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const alwaysOnlineInterval = 4 * time.Minute

func StartAlwaysOnline(ctx context.Context, client *whatsmeow.Client, log *slog.Logger) {
	go func() {
		ticker := time.NewTicker(alwaysOnlineInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if client == nil || !client.IsConnected() {
					continue
				}
				if err := client.SendPresence(ctx, types.PresenceAvailable); err != nil {
					log.Warn("always-online presence failed", slog.Any("err", err))
				}
			}
		}
	}()
}

func ScheduleMarkRead(client *whatsmeow.Client, msgID types.MessageID, ts time.Time, chat, sender types.JID, log *slog.Logger) {
	if client == nil {
		return
	}
	time.AfterFunc(time.Second, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.MarkRead(ctx, []types.MessageID{msgID}, ts, chat, sender); err != nil {
			log.Warn("mark read failed",
				slog.String("msg_id", msgID),
				slog.Any("err", err),
			)
		}
	})
}

func HandleCallReject(ctx context.Context, client *whatsmeow.Client, evt *events.CallOffer, msgRejectCall string, log *slog.Logger) error {
	if client == nil || evt == nil {
		return nil
	}
	if err := client.RejectCall(ctx, evt.From, evt.CallID); err != nil {
		log.Warn("reject call failed",
			slog.String("call_id", evt.CallID),
			slog.Any("err", err),
		)
		return err
	}
	if msgRejectCall == "" {
		return nil
	}
	msg := &waE2E.Message{Conversation: proto.String(msgRejectCall)}
	if _, err := client.SendMessage(ctx, evt.From, msg); err != nil {
		log.Warn("reject call notice send failed",
			slog.String("call_id", evt.CallID),
			slog.Any("err", err),
		)
		return err
	}
	return nil
}
