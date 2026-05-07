package command

import (
	"context"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	apperrors "github.com/canove/whaticket-community/worker/internal/platform/errors"
	"github.com/canove/whaticket-community/worker/internal/rmq"
)

func (h *Handlers) handleDeleteMessage(ctx context.Context, env rmq.Envelope) (any, error) {
	var req DeleteMessageReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}

	msgID := types.MessageID(req.ID)
	revoke := sess.Client.BuildRevoke(recipient, types.EmptyJID, msgID)
	resp, sendErr := sess.Client.SendMessage(ctx, recipient, revoke)
	if sendErr != nil {
		return nil, mapSendError(sendErr)
	}
	return SendResp{ID: string(resp.ID), Timestamp: resp.Timestamp.Unix()}, nil
}

func (h *Handlers) handleEditMessage(ctx context.Context, env rmq.Envelope) (any, error) {
	var req EditMessageReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}

	body := req.Body
	newContent := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: &body,
	}}
	editMsg := sess.Client.BuildEdit(recipient, types.MessageID(req.ID), newContent)
	resp, sendErr := sess.Client.SendMessage(ctx, recipient, editMsg)
	if sendErr != nil {
		return nil, mapSendError(sendErr)
	}
	return SendResp{ID: string(resp.ID), Timestamp: resp.Timestamp.Unix()}, nil
}

func (h *Handlers) handleReactMessage(ctx context.Context, env rmq.Envelope) (any, error) {
	var req ReactMessageReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	recipient, err := resolveJID(req.To)
	if err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}

	participant := types.EmptyJID
	if !req.FromMe && req.Participant != "" {
		jid, parseErr := resolveJID(req.Participant)
		if parseErr != nil {
			return nil, parseErr
		}
		participant = jid
	}

	reactionMsg := sess.Client.BuildReaction(recipient, participant, types.MessageID(req.ID), req.Emoji)
	if reactionMsg.GetReactionMessage() != nil {
		reactionMsg.ReactionMessage.SenderTimestampMS = proto.Int64(time.Now().UnixMilli())
		reactionMsg.ReactionMessage.Key.FromMe = proto.Bool(req.FromMe)
	}

	resp, sendErr := sess.Client.SendMessage(ctx, recipient, reactionMsg)
	if sendErr != nil {
		return nil, mapSendError(sendErr)
	}
	return SendResp{ID: string(resp.ID), Timestamp: resp.Timestamp.Unix()}, nil
}

func (h *Handlers) handleMarkRead(ctx context.Context, env rmq.Envelope) (any, error) {
	var req MarkReadReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	chatJID, err := resolveJID(req.ChatJID)
	if err != nil {
		return nil, err
	}
	senderJID := chatJID
	if req.SenderJID != "" {
		senderJID, err = resolveJID(req.SenderJID)
		if err != nil {
			return nil, err
		}
	}
	if len(req.IDs) == 0 {
		return nil, apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}

	ids := make([]types.MessageID, len(req.IDs))
	for i, id := range req.IDs {
		ids[i] = types.MessageID(id)
	}
	ts := time.UnixMilli(req.TimestampMS)
	if req.TimestampMS == 0 {
		ts = time.Now()
	}

	if err := sess.Client.MarkRead(ctx, ids, ts, chatJID, senderJID); err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrInternal, http.StatusInternalServerError)
	}
	return struct{}{}, nil
}
