package command

import (
	"context"
	"net/http"
	"strings"

	"go.mau.fi/whatsmeow"

	apperrors "github.com/jobasfernandes/whaticket-go-worker/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-worker/internal/rmq"
)

const pairPhoneClientName = "Chrome (Linux)"

func (h *Handlers) handlePairPhone(ctx context.Context, env rmq.Envelope) (any, error) {
	var req PairPhoneReq
	if err := env.Decode(&req); err != nil {
		return nil, err
	}
	sess, ok := h.Mgr.Get(req.WhatsappID)
	if !ok || sess == nil || sess.Client == nil {
		return nil, apperrors.New(apperrors.ErrNoSession, http.StatusNotFound)
	}
	if sess.Client.IsLoggedIn() {
		return nil, apperrors.New(apperrors.ErrAlreadyPaired, http.StatusConflict)
	}

	phone := strings.TrimPrefix(strings.TrimSpace(req.Phone), "+")
	if phone == "" {
		return nil, apperrors.New(apperrors.ErrInvalidPhone, http.StatusBadRequest)
	}

	code, err := sess.Client.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, pairPhoneClientName)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrInvalidPhone, http.StatusBadRequest)
	}

	h.publishInfo(ctx, req.WhatsappID, "pairphone.code", map[string]string{"code": code})
	return PairPhoneResp{Code: code}, nil
}
