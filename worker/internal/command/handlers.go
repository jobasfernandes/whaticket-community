package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.mau.fi/whatsmeow/store/sqlstore"

	apperrors "github.com/canove/whaticket-community/worker/internal/platform/errors"
	"github.com/canove/whaticket-community/worker/internal/rmq"
	whatsmeowpkg "github.com/canove/whaticket-community/worker/internal/whatsmeow"
)

type Handlers struct {
	Mgr        *whatsmeowpkg.Manager
	Runtime    whatsmeowpkg.SessionRuntime
	RMQ        *rmq.Client
	Container  *sqlstore.Container
	DeviceMeta *whatsmeowpkg.DeviceMetaStore
	Log        *slog.Logger
}

func New(mgr *whatsmeowpkg.Manager, runtime whatsmeowpkg.SessionRuntime, rmqClient *rmq.Client, container *sqlstore.Container, meta *whatsmeowpkg.DeviceMetaStore, log *slog.Logger) *Handlers {
	if log == nil {
		log = slog.Default()
	}
	return &Handlers{
		Mgr:        mgr,
		Runtime:    runtime,
		RMQ:        rmqClient,
		Container:  container,
		DeviceMeta: meta,
		Log:        log,
	}
}

func (h *Handlers) handleSessionStart(ctx context.Context, env rmq.Envelope) error {
	var req SessionStartReq
	if err := env.Decode(&req); err != nil {
		return err
	}
	if req.WhatsappID <= 0 {
		return apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}
	if h.Mgr.IsConnected(req.WhatsappID) {
		h.publishInfo(ctx, req.WhatsappID, "SessionAlreadyRunning", map[string]any{
			"whatsappId": req.WhatsappID,
		})
		return nil
	}
	cfg := whatsmeowpkg.StartConfig{
		ConnectionID:     req.WhatsappID,
		JID:              req.JID,
		AdvancedSettings: req.AdvancedSettings,
		MediaMode:        req.MediaMode,
		ProxyURL:         req.ProxyURL,
	}
	if err := h.Mgr.StartSession(ctx, cfg, h.Runtime); err != nil {
		h.Log.Error("session start failed",
			slog.Int("conn_id", req.WhatsappID),
			slog.Any("err", err),
		)
		return err
	}
	h.Log.Info("session started",
		slog.Int("conn_id", req.WhatsappID),
		slog.String("jid", req.JID),
	)
	return nil
}

func (h *Handlers) handleSessionStop(ctx context.Context, env rmq.Envelope) error {
	var req SessionStopReq
	if err := env.Decode(&req); err != nil {
		return err
	}
	h.Mgr.SendKill(req.WhatsappID)
	h.Log.Info("session stop requested", slog.Int("conn_id", req.WhatsappID))
	_ = ctx
	return nil
}

func (h *Handlers) handleSessionLogout(ctx context.Context, env rmq.Envelope) error {
	var req SessionLogoutReq
	if err := env.Decode(&req); err != nil {
		return err
	}

	sess, ok := h.Mgr.Get(req.WhatsappID)
	var deviceJID string
	if ok && sess != nil && sess.Client != nil {
		if sess.Client.IsConnected() {
			if err := sess.Client.Logout(ctx); err != nil {
				h.Log.Warn("logout failed",
					slog.Int("conn_id", req.WhatsappID),
					slog.Any("err", err),
				)
			}
		}
		if sess.Client.Store != nil && sess.Client.Store.ID != nil {
			deviceJID = sess.Client.Store.ID.String()
			if err := sess.Client.Store.Delete(ctx); err != nil {
				h.Log.Warn("device store delete failed",
					slog.Int("conn_id", req.WhatsappID),
					slog.String("jid", deviceJID),
					slog.Any("err", err),
				)
			}
		}
	}

	h.Mgr.SendKill(req.WhatsappID)

	if deviceJID != "" {
		if err := h.DeviceMeta.Delete(deviceJID); err != nil {
			h.Log.Warn("device meta delete failed",
				slog.Int("conn_id", req.WhatsappID),
				slog.String("jid", deviceJID),
				slog.Any("err", err),
			)
		}
		return nil
	}

	if err := h.DeviceMeta.DeleteByConnID(req.WhatsappID); err != nil {
		h.Log.Warn("device meta delete by conn failed",
			slog.Int("conn_id", req.WhatsappID),
			slog.Any("err", err),
		)
	}
	return nil
}

func (h *Handlers) handleSessionUpdateSettings(ctx context.Context, env rmq.Envelope) error {
	var req SessionUpdateSettingsReq
	if err := env.Decode(&req); err != nil {
		return err
	}
	if !h.Mgr.UpdateSettings(req.WhatsappID, req.AdvancedSettings, req.MediaMode) {
		return apperrors.New(apperrors.ErrNoSession, http.StatusNotFound)
	}
	h.Log.Info("session settings updated",
		slog.Int("conn_id", req.WhatsappID),
		slog.String("media_mode", req.MediaMode),
	)
	_ = ctx
	return nil
}

func (h *Handlers) publishInfo(ctx context.Context, connID int, eventType string, payload any) {
	if h.RMQ == nil {
		return
	}
	env, err := rmq.WrapPayload(eventType, connID, payload)
	if err != nil {
		h.Log.Warn("publish info wrap failed",
			slog.Int("conn_id", connID),
			slog.String("event_type", eventType),
			slog.Any("err", err),
		)
		return
	}
	routingKey := fmt.Sprintf("wa.event.%d.%s", connID, eventType)
	if err := h.RMQ.Publish(ctx, exchangeWaEvents, routingKey, env); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		h.Log.Warn("publish info failed",
			slog.Int("conn_id", connID),
			slog.String("event_type", eventType),
			slog.Any("err", err),
		)
	}
}
