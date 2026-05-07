package waevents

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

type ReceiptPayload struct {
	Event ReceiptEvent `json:"event"`
	State string       `json:"state"`
}

type ReceiptEvent struct {
	MessageIDs []string `json:"MessageIDs"`
	Sender     string   `json:"Sender"`
	Chat       string   `json:"Chat"`
}

const (
	ackDelivered = 2
	ackRead      = 3
)

var ackRetryBackoffs = []time.Duration{
	0,
	100 * time.Millisecond,
	300 * time.Millisecond,
	900 * time.Millisecond,
}

func (c *Consumer) handleReceipt(ctx context.Context, whatsappID uint, payloadRaw []byte) error {
	var payload ReceiptPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		c.Log.Warn("waevents receipt decode failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return nil
	}

	ack, ok := receiptStateToAck(payload.State)
	if !ok {
		c.Log.Debug("waevents receipt state ignored",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.String("state", payload.State),
		)
		return nil
	}

	for _, msgID := range payload.Event.MessageIDs {
		if msgID == "" {
			continue
		}
		if err := c.updateAckWithRetry(ctx, msgID, ack); err != nil {
			c.Log.Warn("waevents update ack failed",
				slog.String("message_id", msgID),
				slog.Int("ack", ack),
				slog.Any("err", err),
			)
		}
	}
	return nil
}

func receiptStateToAck(state string) (int, bool) {
	switch state {
	case "Read", "ReadSelf":
		return ackRead, true
	case "Delivered":
		return ackDelivered, true
	default:
		return 0, false
	}
}

func (c *Consumer) updateAckWithRetry(ctx context.Context, msgID string, ack int) error {
	for attempt, backoff := range ackRetryBackoffs {
		if backoff > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		result := c.DB.WithContext(ctx).Exec("UPDATE messages SET ack = ? WHERE id = ?", ack, msgID)
		if result.Error != nil {
			c.Log.Warn("waevents ack update sql error",
				slog.String("message_id", msgID),
				slog.Int("attempt", attempt+1),
				slog.Any("err", result.Error),
			)
			continue
		}

		if result.RowsAffected > 0 {
			var ticketID uint
			row := c.DB.WithContext(ctx).Raw("SELECT ticket_id FROM messages WHERE id = ?", msgID).Scan(&ticketID)
			if row.Error != nil || ticketID == 0 {
				c.Log.Warn("waevents ack ticket lookup failed",
					slog.String("message_id", msgID),
					slog.Any("err", row.Error),
				)
				return nil
			}
			c.WS.Publish(ticketChannel(ticketID), wsAppMessageUpdate, map[string]any{
				"id":  msgID,
				"ack": ack,
			})
			return nil
		}
	}

	c.Log.Warn("waevents ack message not found after retries",
		slog.String("message_id", msgID),
		slog.Int("ack", ack),
	)
	return nil
}
