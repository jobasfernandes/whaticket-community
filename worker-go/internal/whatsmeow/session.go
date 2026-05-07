package whatsmeow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	wa "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

const (
	connectMaxAttempts = 3
	connectRetryUnit   = 5 * time.Second
)

type StartConfig struct {
	ConnectionID     int
	JID              string
	AdvancedSettings AdvancedSettings
	MediaMode        string
	ProxyURL         string
}

type SessionRuntime struct {
	Container    *sqlstore.Container
	EventHandler *EventHandler
	DeviceMeta   *DeviceMetaStore
	WALog        waLog.Logger
}

func (m *Manager) StartSession(ctx context.Context, cfg StartConfig, runtime SessionRuntime) error {
	if runtime.Container == nil {
		return fmt.Errorf("session: container is nil")
	}
	if runtime.EventHandler == nil {
		return fmt.Errorf("session: event handler is nil")
	}
	if _, exists := m.Get(cfg.ConnectionID); exists {
		return fmt.Errorf("session: connection %d already running", cfg.ConnectionID)
	}
	if !m.TryStartConnect(cfg.ConnectionID) {
		return fmt.Errorf("session: connection %d already connecting", cfg.ConnectionID)
	}

	m.handlerWG.Add(1)
	go m.sessionGoroutine(ctx, cfg, runtime)
	return nil
}

func (m *Manager) sessionGoroutine(parentCtx context.Context, cfg StartConfig, runtime SessionRuntime) {
	defer m.handlerWG.Done()
	defer m.CleanupKill(cfg.ConnectionID)
	defer m.Delete(cfg.ConnectionID)
	defer m.ClearConnecting(cfg.ConnectionID)

	sessionCtx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	device, err := m.resolveDevice(sessionCtx, cfg, runtime.Container)
	if err != nil {
		m.log.Error("resolve device failed",
			slog.Int("conn_id", cfg.ConnectionID),
			slog.String("jid", cfg.JID),
			slog.Any("err", err),
		)
		runtime.EventHandler.publish(parentCtx, cfg.ConnectionID, "ConnectFailure", map[string]any{
			"reason": "resolve_device",
			"error":  err.Error(),
		})
		return
	}

	waLogger := runtime.WALog
	if waLogger == nil {
		waLogger = waLog.Noop
	}
	client := NewClientForDevice(device, waLogger)

	if cfg.ProxyURL != "" {
		if err := client.SetProxyAddress(cfg.ProxyURL); err != nil {
			m.log.Warn("set proxy failed",
				slog.Int("conn_id", cfg.ConnectionID),
				slog.String("proxy", cfg.ProxyURL),
				slog.Any("err", err),
			)
		}
	}

	handlerID := client.AddEventHandler(runtime.EventHandler.Handle(cfg.ConnectionID))

	m.Set(cfg.ConnectionID, &Session{
		ConnectionID: cfg.ConnectionID,
		Client:       client,
		HandlerID:    handlerID,
		Settings:     cfg.AdvancedSettings,
		MediaMode:    cfg.MediaMode,
		Cancel:       cancel,
	})

	killCh := m.RegisterKill(cfg.ConnectionID)

	var pairingOK bool
	if client.Store.ID == nil {
		pairingOK = m.runQRPairing(sessionCtx, cfg, runtime, client)
	} else {
		pairingOK = m.runReconnect(sessionCtx, cfg, runtime, client)
	}
	if !pairingOK {
		return
	}

	if cfg.AdvancedSettings.AlwaysOnline && client.IsConnected() {
		StartAlwaysOnline(sessionCtx, client, m.log)
	}

	select {
	case <-killCh:
		m.log.Info("session kill signal received",
			slog.Int("conn_id", cfg.ConnectionID),
		)
	case <-sessionCtx.Done():
		m.log.Info("session context cancelled",
			slog.Int("conn_id", cfg.ConnectionID),
			slog.Any("err", sessionCtx.Err()),
		)
	}
}

func (m *Manager) resolveDevice(ctx context.Context, cfg StartConfig, container *sqlstore.Container) (*store.Device, error) {
	if cfg.JID != "" {
		jid, ok := parseJID(cfg.JID)
		if !ok {
			return nil, fmt.Errorf("invalid jid %q", cfg.JID)
		}
		device, err := container.GetDevice(ctx, jid)
		if err == nil && device != nil {
			return device, nil
		}
		if err != nil {
			m.log.Warn("get device fallback to new",
				slog.String("jid", cfg.JID),
				slog.Any("err", err),
			)
		}
	}
	return container.NewDevice(), nil
}

func (m *Manager) runQRPairing(ctx context.Context, cfg StartConfig, runtime SessionRuntime, client *wa.Client) bool {
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		m.log.Error("get qr channel failed",
			slog.Int("conn_id", cfg.ConnectionID),
			slog.Any("err", err),
		)
		runtime.EventHandler.publish(ctx, cfg.ConnectionID, "ConnectFailure", map[string]any{
			"reason": "qr_channel",
			"error":  err.Error(),
		})
		return false
	}

	if err := client.Connect(); err != nil {
		m.log.Error("client connect failed during qr",
			slog.Int("conn_id", cfg.ConnectionID),
			slog.Any("err", err),
		)
		runtime.EventHandler.publish(ctx, cfg.ConnectionID, "ConnectFailure", map[string]any{
			"reason": "connect",
			"error":  err.Error(),
		})
		return false
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			runtime.EventHandler.publish(ctx, cfg.ConnectionID, "qr.code", map[string]any{
				"code":    evt.Code,
				"timeout": evt.Timeout.Milliseconds(),
			})
		case "timeout":
			runtime.EventHandler.publish(ctx, cfg.ConnectionID, "qr.timeout", map[string]any{})
			m.SendKill(cfg.ConnectionID)
			return false
		case "success":
			runtime.EventHandler.publish(ctx, cfg.ConnectionID, "qr.success", map[string]any{})
			if client.Store != nil && client.Store.ID != nil && runtime.DeviceMeta != nil {
				if err := runtime.DeviceMeta.Set(client.Store.ID.String(), cfg.ConnectionID); err != nil {
					m.log.Error("device meta set after pair failed",
						slog.Int("conn_id", cfg.ConnectionID),
						slog.Any("err", err),
					)
				}
			}
			return true
		case "err-client-outdated", "err-scanned-without-multidevice", "err-unexpected-state":
			runtime.EventHandler.publish(ctx, cfg.ConnectionID, "qr.error", map[string]any{
				"event": evt.Event,
			})
			m.SendKill(cfg.ConnectionID)
			return false
		default:
			runtime.EventHandler.publish(ctx, cfg.ConnectionID, "qr."+evt.Event, map[string]any{
				"event": evt.Event,
			})
		}
	}
	return false
}

func (m *Manager) runReconnect(ctx context.Context, cfg StartConfig, runtime SessionRuntime, client *wa.Client) bool {
	var lastErr error
	for attempt := 1; attempt <= connectMaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return false
		}
		if err := client.Connect(); err != nil {
			lastErr = err
			m.log.Warn("reconnect attempt failed",
				slog.Int("conn_id", cfg.ConnectionID),
				slog.Int("attempt", attempt),
				slog.Any("err", err),
			)
			if attempt == connectMaxAttempts {
				break
			}
			t := time.NewTimer(time.Duration(attempt) * connectRetryUnit)
			select {
			case <-ctx.Done():
				t.Stop()
				return false
			case <-t.C:
			}
			continue
		}
		if client.Store != nil && client.Store.ID != nil && runtime.DeviceMeta != nil {
			if err := runtime.DeviceMeta.Set(client.Store.ID.String(), cfg.ConnectionID); err != nil {
				m.log.Error("device meta set after reconnect failed",
					slog.Int("conn_id", cfg.ConnectionID),
					slog.Any("err", err),
				)
			}
		}
		runtime.EventHandler.publish(ctx, cfg.ConnectionID, "SessionRestored", map[string]any{
			"jid": jidString(client),
		})
		return true
	}

	reason := "exhausted"
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	runtime.EventHandler.publish(ctx, cfg.ConnectionID, "ConnectFailure", map[string]any{
		"reason": reason,
		"error":  errMsg,
	})
	return false
}

func (m *Manager) AutoRestoreFromContainer(ctx context.Context, runtime SessionRuntime) error {
	if runtime.Container == nil {
		return fmt.Errorf("auto restore: container is nil")
	}
	if runtime.DeviceMeta == nil {
		return fmt.Errorf("auto restore: device meta is nil")
	}
	devices, err := runtime.Container.GetAllDevices(ctx)
	if err != nil {
		return fmt.Errorf("auto restore: get all devices: %w", err)
	}
	for _, device := range devices {
		if device == nil || device.ID == nil {
			continue
		}
		jid := device.ID.String()
		connID, ok, err := runtime.DeviceMeta.GetConnID(jid)
		if err != nil {
			m.log.Error("auto restore: lookup conn id failed",
				slog.String("jid", jid),
				slog.Any("err", err),
			)
			continue
		}
		if !ok {
			m.log.Warn("auto restore: orphan device in sqlstore",
				slog.String("jid", jid),
			)
			continue
		}
		cfg := StartConfig{
			ConnectionID:     connID,
			JID:              jid,
			AdvancedSettings: AdvancedSettings{},
			MediaMode:        "base64",
		}
		if err := m.StartSession(ctx, cfg, runtime); err != nil {
			m.log.Error("auto restore: start session failed",
				slog.Int("conn_id", connID),
				slog.String("jid", jid),
				slog.Any("err", err),
			)
		}
	}
	return nil
}

func jidString(client *wa.Client) string {
	if client == nil || client.Store == nil || client.Store.ID == nil {
		return ""
	}
	return client.Store.ID.String()
}
