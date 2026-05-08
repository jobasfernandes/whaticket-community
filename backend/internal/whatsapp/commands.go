package whatsapp

import (
	"context"

	"github.com/canove/whaticket-community/backend/internal/rmq"
)

const (
	exchangeWaCommands     = "wa.commands"
	routingSessionStart    = "session.start"
	routingSessionStop     = "session.stop"
	routingSessionLogout   = "session.logout"
	routingSessionSettings = "session.update_settings"
)

type startSessionPayload struct {
	WhatsappID       uint             `json:"whatsappId"`
	AdvancedSettings AdvancedSettings `json:"advancedSettings"`
	MediaMode        string           `json:"mediaMode"`
}

type stopSessionPayload struct {
	WhatsappID uint `json:"whatsappId"`
}

type logoutPayload struct {
	WhatsappID uint `json:"whatsappId"`
}

type updateSettingsPayload struct {
	WhatsappID       uint             `json:"whatsappId"`
	AdvancedSettings AdvancedSettings `json:"advancedSettings"`
	MediaMode        string           `json:"mediaMode"`
}

func PublishStartSession(ctx context.Context, pub RMQPublisher, w *Whatsapp) error {
	mediaMode := MediaDeliveryS3
	payload := startSessionPayload{
		WhatsappID:       w.ID,
		AdvancedSettings: w.AdvancedSettings,
		MediaMode:        mediaMode,
	}
	env, err := rmq.WrapPayload(routingSessionStart, int(w.ID), payload)
	if err != nil {
		return err
	}
	return pub.Publish(ctx, exchangeWaCommands, routingSessionStart, env)
}

func PublishStopSession(ctx context.Context, pub RMQPublisher, whatsappID uint) error {
	env, err := rmq.WrapPayload(routingSessionStop, int(whatsappID), stopSessionPayload{WhatsappID: whatsappID})
	if err != nil {
		return err
	}
	return pub.Publish(ctx, exchangeWaCommands, routingSessionStop, env)
}

func PublishLogout(ctx context.Context, pub RMQPublisher, whatsappID uint) error {
	env, err := rmq.WrapPayload(routingSessionLogout, int(whatsappID), logoutPayload{WhatsappID: whatsappID})
	if err != nil {
		return err
	}
	return pub.Publish(ctx, exchangeWaCommands, routingSessionLogout, env)
}

func PublishUpdateSettings(ctx context.Context, pub RMQPublisher, w *Whatsapp) error {
	mediaMode := MediaDeliveryS3
	payload := updateSettingsPayload{
		WhatsappID:       w.ID,
		AdvancedSettings: w.AdvancedSettings,
		MediaMode:        mediaMode,
	}
	env, err := rmq.WrapPayload(routingSessionSettings, int(w.ID), payload)
	if err != nil {
		return err
	}
	return pub.Publish(ctx, exchangeWaCommands, routingSessionSettings, env)
}
