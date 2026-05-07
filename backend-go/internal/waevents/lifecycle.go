package waevents

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"gorm.io/gorm"
)

type qrCodePayload struct {
	Code string `json:"code"`
}

type pairphoneCodePayload struct {
	Code string `json:"code"`
}

const (
	statusQRCode       = "qrcode"
	statusOpening      = "OPENING"
	statusConnected    = "CONNECTED"
	statusDisconnected = "DISCONNECTED"
)

const (
	wsNotificationChannel    = "notification"
	wsWhatsappSessionUpdate  = "whatsappSession.update"
	wsWhatsappSessionPairing = "whatsappSession.pairphone"
	wsAppMessageUpdate       = "appMessage.update"
)

func (c *Consumer) emitWhatsappSessionUpdate(ctx context.Context, whatsappID uint) error {
	w, err := c.WhatsappSvc.Show(ctx, whatsappID)
	if err != nil {
		if isWhatsappNotFound(err) {
			c.Log.Warn("waevents row gone; asking worker to cleanup",
				slog.Uint64("whatsapp_id", uint64(whatsappID)),
			)
			if perr := c.WhatsappSvc.PublishLogout(ctx, whatsappID); perr != nil {
				c.Log.Warn("waevents publish logout failed",
					slog.Uint64("whatsapp_id", uint64(whatsappID)),
					slog.Any("err", perr),
				)
			}
			return nil
		}
		c.Log.Warn("waevents reload whatsapp failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return err
	}
	c.WS.Publish(wsNotificationChannel, wsWhatsappSessionUpdate, c.WhatsappSvc.SerializeForWS(w))
	return nil
}

func isWhatsappNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ERR_WAPP_NOT_FOUND")
}

func (c *Consumer) handleQRCode(ctx context.Context, whatsappID uint, payload []byte) error {
	var p qrCodePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		c.Log.Warn("waevents qr.code decode failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return nil
	}
	if err := c.WhatsappSvc.UpdateStatus(ctx, whatsappID, statusQRCode, p.Code); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleQRTimeout(ctx context.Context, whatsappID uint, _ []byte) error {
	if err := c.WhatsappSvc.UpdateStatus(ctx, whatsappID, statusDisconnected, ""); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleQRSuccess(_ context.Context, whatsappID uint, _ []byte) error {
	c.Log.Info("waevents qr success",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
	)
	return nil
}

func (c *Consumer) handlePairphoneCode(_ context.Context, whatsappID uint, payload []byte) error {
	var p pairphoneCodePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		c.Log.Warn("waevents pairphone.code decode failed",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
			slog.Any("err", err),
		)
		return nil
	}
	c.WS.Publish(wsNotificationChannel, wsWhatsappSessionPairing, map[string]any{
		"whatsappId": whatsappID,
		"code":       p.Code,
	})
	return nil
}

func (c *Consumer) handlePairSuccess(_ context.Context, whatsappID uint, _ []byte) error {
	c.Log.Info("waevents pair success",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
	)
	return nil
}

func (c *Consumer) handleConnected(ctx context.Context, whatsappID uint, _ []byte) error {
	if err := c.WhatsappSvc.UpdateConnected(ctx, whatsappID); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleDisconnected(_ context.Context, whatsappID uint, _ []byte) error {
	c.Log.Info("waevents disconnected (transient)",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
	)
	return nil
}

func (c *Consumer) handleLoggedOut(ctx context.Context, whatsappID uint, _ []byte) error {
	if err := c.WhatsappSvc.UpdateDisconnected(ctx, whatsappID); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleStreamReplaced(ctx context.Context, whatsappID uint, _ []byte) error {
	if err := c.WhatsappSvc.UpdateRetries(ctx, whatsappID); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleTemporaryBan(ctx context.Context, whatsappID uint, payload []byte) error {
	c.Log.Warn("waevents temporary ban",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
		slog.String("payload", string(payload)),
	)
	if err := c.WhatsappSvc.UpdateRetries(ctx, whatsappID); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleConnectFailure(ctx context.Context, whatsappID uint, payload []byte) error {
	c.Log.Warn("waevents connect failure",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
		slog.String("payload", string(payload)),
	)
	if err := c.WhatsappSvc.UpdateRetries(ctx, whatsappID); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleKeepAliveTimeout(_ context.Context, whatsappID uint, payload []byte) error {
	c.Log.Warn("waevents keep alive timeout",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
		slog.String("payload", string(payload)),
	)
	return nil
}

func (c *Consumer) handleKeepAliveRestored(_ context.Context, whatsappID uint, _ []byte) error {
	c.Log.Info("waevents keep alive restored",
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
	)
	return nil
}

func (c *Consumer) handleSessionRestored(ctx context.Context, whatsappID uint, _ []byte) error {
	w, err := c.WhatsappSvc.Show(ctx, whatsappID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Log.Info("waevents session restored for missing row, force logout",
				slog.Uint64("whatsapp_id", uint64(whatsappID)),
			)
			if perr := c.WhatsappSvc.PublishLogout(ctx, whatsappID); perr != nil {
				c.Log.Warn("waevents publish logout failed",
					slog.Uint64("whatsapp_id", uint64(whatsappID)),
					slog.Any("err", perr),
				)
			}
			return nil
		}
		return err
	}

	if w.GetStatus() == statusDisconnected {
		c.Log.Info("waevents session restored for disconnected row, stopping orphan",
			slog.Uint64("whatsapp_id", uint64(whatsappID)),
		)
		if perr := c.WhatsappSvc.PublishStopSession(ctx, whatsappID); perr != nil {
			c.Log.Warn("waevents publish stop session failed",
				slog.Uint64("whatsapp_id", uint64(whatsappID)),
				slog.Any("err", perr),
			)
		}
		return nil
	}

	if err := c.WhatsappSvc.UpdateStatus(ctx, whatsappID, statusOpening, ""); err != nil {
		return err
	}
	return c.emitWhatsappSessionUpdate(ctx, whatsappID)
}

func (c *Consumer) handleNoOp(_ context.Context, whatsappID uint, eventName string, _ []byte) error {
	c.Log.Debug("waevents ignored event",
		slog.String("event", eventName),
		slog.Uint64("whatsapp_id", uint64(whatsappID)),
	)
	return nil
}
