package ws

import (
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/canove/whaticket-community/backend/internal/auth"
)

func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	if h.shuttingDown.Load() {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseAccessToken(token, h.cfg.JWTSecret)
	if err != nil || claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	acceptOpts := &websocket.AcceptOptions{}
	if h.cfg.AllowedOrigin != "" {
		acceptOpts.OriginPatterns = []string{h.cfg.AllowedOrigin}
	}

	conn, err := websocket.Accept(w, r, acceptOpts)
	if err != nil {
		h.log.Warn("ws upgrade failed",
			"userId", claims.ID,
			"err", err,
		)
		return
	}

	client := newClient(h, conn, claims.ID, claims.Profile, h.log)
	h.registerClient(client)

	startedAt := time.Now()
	h.log.Info("ws connection established",
		"userId", claims.ID,
		"profile", claims.Profile,
	)

	defer func() {
		_ = conn.Close(websocket.StatusInternalError, "")
		h.log.Info("ws connection closed",
			"userId", claims.ID,
			"profile", claims.Profile,
			"durationMs", time.Since(startedAt).Milliseconds(),
		)
	}()

	client.sendSystem(EventConnected, map[string]any{
		"userId":  claims.ID,
		"profile": claims.Profile,
	})

	go client.writePump(r.Context(), h.cfg.PingInterval, h.cfg.WriteTimeout, h.cfg.PongTimeout)
	client.readPump(r.Context(), h.cfg.TicketAuthz)
	client.close()
}
