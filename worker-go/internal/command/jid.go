package command

import (
	"net/http"

	"go.mau.fi/whatsmeow/types"

	apperrors "github.com/jobasfernandes/whaticket-go-worker/internal/platform/errors"
	whatsmeowpkg "github.com/jobasfernandes/whaticket-go-worker/internal/whatsmeow"
)

func resolveJID(raw string) (types.JID, error) {
	jid, ok := whatsmeowpkg.ParseJID(raw)
	if !ok {
		return types.JID{}, apperrors.New(apperrors.ErrInvalidPhone, http.StatusBadRequest)
	}
	return jid, nil
}

func resolveJIDs(raws []string) ([]types.JID, error) {
	out := make([]types.JID, 0, len(raws))
	for _, raw := range raws {
		jid, err := resolveJID(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, jid)
	}
	return out, nil
}
