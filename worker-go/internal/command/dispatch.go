package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jobasfernandes/whaticket-go-worker/internal/rmq"
)

const (
	queueWorkerCommands = "worker.commands"
	queueWorkerRPC      = "worker.rpc"
	exchangeWaEvents    = "wa.events"
	consumerTagCommands = "worker-cmd"
)

const (
	cmdSessionStart          = "session.start"
	cmdSessionStop           = "session.stop"
	cmdSessionLogout         = "session.logout"
	cmdSessionUpdateSettings = "session.update_settings"
	rpcSessionPairPhone      = "session.pairphone"
	rpcMessageSendText       = "message.send.text"
	rpcMessageSendImage      = "message.send.image"
	rpcMessageSendAudio      = "message.send.audio"
	rpcMessageSendVideo      = "message.send.video"
	rpcMessageSendDocument   = "message.send.document"
	rpcMessageSendSticker    = "message.send.sticker"
	rpcMessageSendContact    = "message.send.contact"
	rpcMessageSendLocation   = "message.send.location"
	rpcMessageDelete         = "message.delete"
	rpcMessageEdit           = "message.edit"
	rpcMessageReact          = "message.react"
	rpcMessageMarkRead       = "message.markread"
	rpcContactCheckNumber    = "contact.checkNumber"
	rpcContactGetProfilePic  = "contact.getProfilePic"
	rpcContactGetInfo        = "contact.getInfo"
	rpcPresenceSend          = "presence.send"
	rpcChatPresenceSend      = "chat.presence.send"
)

func (h *Handlers) Register(ctx context.Context) error {
	if h.RMQ == nil {
		return errors.New("command: rmq client is nil")
	}

	go func() {
		if err := h.RMQ.Consume(ctx, queueWorkerCommands, consumerTagCommands, h.routeCommand); err != nil {
			if !errors.Is(err, context.Canceled) {
				h.Log.Error("commands consumer ended", slog.Any("err", err))
			}
		}
	}()

	rpcHandlers := h.rpcHandlers()
	go func() {
		if err := h.RMQ.ServeRPC(ctx, queueWorkerRPC, rpcHandlers); err != nil {
			if !errors.Is(err, context.Canceled) {
				h.Log.Error("rpc server ended", slog.Any("err", err))
			}
		}
	}()

	h.Log.Info("command handlers registered",
		slog.String("commands_queue", queueWorkerCommands),
		slog.String("rpc_queue", queueWorkerRPC),
	)
	return nil
}

func (h *Handlers) routeCommand(ctx context.Context, env rmq.Envelope) error {
	switch env.Type {
	case cmdSessionStart:
		return h.handleSessionStart(ctx, env)
	case cmdSessionStop:
		return h.handleSessionStop(ctx, env)
	case cmdSessionLogout:
		return h.handleSessionLogout(ctx, env)
	case cmdSessionUpdateSettings:
		return h.handleSessionUpdateSettings(ctx, env)
	default:
		h.Log.Debug("unknown command type", slog.String("type", env.Type))
		return nil
	}
}

func (h *Handlers) rpcHandlers() map[string]rmq.RPCHandlerFunc {
	return map[string]rmq.RPCHandlerFunc{
		rpcSessionPairPhone:     h.handlePairPhone,
		rpcMessageSendText:      h.handleSendText,
		rpcMessageSendImage:     h.handleSendImage,
		rpcMessageSendAudio:     h.handleSendAudio,
		rpcMessageSendVideo:     h.handleSendVideo,
		rpcMessageSendDocument:  h.handleSendDocument,
		rpcMessageSendSticker:   h.handleSendSticker,
		rpcMessageSendContact:   h.handleSendContact,
		rpcMessageSendLocation:  h.handleSendLocation,
		rpcMessageDelete:        h.handleDeleteMessage,
		rpcMessageEdit:          h.handleEditMessage,
		rpcMessageReact:         h.handleReactMessage,
		rpcMessageMarkRead:      h.handleMarkRead,
		rpcContactCheckNumber:   h.handleCheckNumber,
		rpcContactGetProfilePic: h.handleGetProfilePic,
		rpcContactGetInfo:       h.handleGetContactInfo,
		rpcPresenceSend:         h.handleSendPresence,
		rpcChatPresenceSend:     h.handleSendChatPresence,
	}
}
