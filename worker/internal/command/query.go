package command

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	apperrors "github.com/canove/whaticket-community/worker/internal/platform/errors"
	"github.com/canove/whaticket-community/worker/internal/rmq"
)

func (h *Handlers) handleCheckNumber(ctx context.Context, env rmq.Envelope) (any, error) {
	var req CheckNumberReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	if len(req.Numbers) == 0 {
		return []CheckNumberResult{}, nil
	}
	queries := make([]string, 0, len(req.Numbers))
	for _, raw := range req.Numbers {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "+") {
			trimmed = "+" + trimmed
		}
		queries = append(queries, trimmed)
	}
	if len(queries) == 0 {
		return []CheckNumberResult{}, nil
	}
	results, callErr := sess.Client.IsOnWhatsApp(ctx, queries)
	if callErr != nil {
		return nil, apperrors.Wrap(callErr, apperrors.ErrInternal, http.StatusInternalServerError)
	}
	out := make([]CheckNumberResult, 0, len(results))
	for _, r := range results {
		entry := CheckNumberResult{Number: r.Query, Exists: r.IsIn}
		if r.IsIn {
			entry.JID = r.JID.String()
		}
		out = append(out, entry)
	}
	return out, nil
}

func (h *Handlers) handleGetProfilePic(ctx context.Context, env rmq.Envelope) (any, error) {
	var req GetProfilePicReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	jid, err := resolveJID(req.JID)
	if err != nil {
		return nil, err
	}
	info, picErr := sess.Client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{Preview: req.Preview})
	if picErr != nil {
		if errors.Is(picErr, whatsmeow.ErrProfilePictureNotSet) || errors.Is(picErr, whatsmeow.ErrProfilePictureUnauthorized) {
			return GetProfilePicResp{URL: ""}, nil
		}
		return nil, apperrors.Wrap(picErr, apperrors.ErrInternal, http.StatusBadGateway)
	}
	if info == nil {
		return GetProfilePicResp{URL: ""}, nil
	}
	return GetProfilePicResp{URL: info.URL}, nil
}

func (h *Handlers) handleGetContactInfo(ctx context.Context, env rmq.Envelope) (any, error) {
	var req GetContactInfoReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	if len(req.JIDs) == 0 {
		return []ContactInfo{}, nil
	}
	jids, err := resolveJIDs(req.JIDs)
	if err != nil {
		return nil, err
	}
	infos, callErr := sess.Client.GetUserInfo(ctx, jids)
	if callErr != nil {
		return nil, apperrors.Wrap(callErr, apperrors.ErrInternal, http.StatusBadGateway)
	}
	out := make([]ContactInfo, 0, len(jids))
	for _, jid := range jids {
		info, ok := infos[jid]
		entry := ContactInfo{JID: jid.String()}
		if ok {
			entry.Status = info.Status
			if info.VerifiedName != nil && info.VerifiedName.Details != nil {
				entry.BusinessName = info.VerifiedName.Details.GetVerifiedName()
				entry.PushName = info.VerifiedName.Details.GetVerifiedName()
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

func (h *Handlers) handleSendPresence(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendPresenceReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	state, err := mapPresence(req.Presence)
	if err != nil {
		return nil, err
	}
	if presErr := sess.Client.SendPresence(ctx, state); presErr != nil {
		return nil, apperrors.Wrap(presErr, apperrors.ErrInternal, http.StatusBadGateway)
	}
	return struct{}{}, nil
}

func (h *Handlers) handleSendChatPresence(ctx context.Context, env rmq.Envelope) (any, error) {
	var req SendChatPresenceReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, err := h.requireLiveSession(req.WhatsappID)
	if err != nil {
		return nil, err
	}
	chat, err := resolveJID(req.ChatJID)
	if err != nil {
		return nil, err
	}
	state, mediaState, err := mapChatPresence(req.State)
	if err != nil {
		return nil, err
	}
	if presErr := sess.Client.SendChatPresence(ctx, chat, state, mediaState); presErr != nil {
		return nil, apperrors.Wrap(presErr, apperrors.ErrInternal, http.StatusBadGateway)
	}
	return struct{}{}, nil
}

func mapPresence(raw string) (types.Presence, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "available", "online":
		return types.PresenceAvailable, nil
	case "unavailable", "offline":
		return types.PresenceUnavailable, nil
	default:
		return "", apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}
}

func mapChatPresence(raw string) (types.ChatPresence, types.ChatPresenceMedia, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "composing", "typing":
		return types.ChatPresenceComposing, types.ChatPresenceMediaText, nil
	case "recording":
		return types.ChatPresenceComposing, types.ChatPresenceMediaAudio, nil
	case "paused":
		return types.ChatPresencePaused, types.ChatPresenceMediaText, nil
	default:
		return "", "", apperrors.New(apperrors.ErrInternal, http.StatusBadRequest)
	}
}
