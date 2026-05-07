package whatsapp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/platform/httpx"
)

type Handler struct {
	Deps         *Deps
	Logger       *slog.Logger
	AccessSecret []byte
}

type pairPhoneBody struct {
	Phone string `json:"phone"`
}

type pairPhoneResponseBody struct {
	LinkingCode string `json:"linkingCode"`
	QRCode      string `json:"qrcode"`
}

type messageResponse struct {
	Message string `json:"message"`
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(g chi.Router) {
		g.Use(auth.IsAuth(h.AccessSecret))
		g.Use(auth.IsAdmin)
		g.Get("/whatsapp", httpx.Wrap(h.index))
		g.Get("/whatsapp/{whatsappId}", httpx.Wrap(h.show))
		g.Post("/whatsapp", httpx.Wrap(h.store))
		g.Put("/whatsapp/{whatsappId}", httpx.Wrap(h.update))
		g.Delete("/whatsapp/{whatsappId}", httpx.Wrap(h.remove))

		g.Post("/whatsappsession/{whatsappId}", httpx.Wrap(h.sessionStart))
		g.Put("/whatsappsession/{whatsappId}", httpx.Wrap(h.sessionRestart))
		g.Delete("/whatsappsession/{whatsappId}", httpx.Wrap(h.sessionLogout))
		g.Post("/whatsappsession/{whatsappId}/pairphone", httpx.Wrap(h.sessionPairPhone))
	})
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) error {
	items, appErr := h.Deps.List(r.Context())
	if appErr != nil {
		return appErr
	}
	dtos := make([]WhatsappDTO, 0, len(items))
	for i := range items {
		dtos = append(dtos, Serialize(&items[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, dtos)
	return nil
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseWhatsappID(r)
	if appErr != nil {
		return appErr
	}
	entity, appErr := h.Deps.Show(r.Context(), id)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(entity))
	return nil
}

func (h *Handler) store(w http.ResponseWriter, r *http.Request) error {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	entity, appErr := h.Deps.Create(r.Context(), req)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusCreated, Serialize(entity))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseWhatsappID(r)
	if appErr != nil {
		return appErr
	}
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	entity, appErr := h.Deps.Update(r.Context(), id, req)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, Serialize(entity))
	return nil
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseWhatsappID(r)
	if appErr != nil {
		return appErr
	}
	if err := h.Deps.Delete(r.Context(), id); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
	return nil
}

func (h *Handler) sessionStart(w http.ResponseWriter, r *http.Request) error {
	return h.startOrRestart(w, r, false)
}

func (h *Handler) sessionRestart(w http.ResponseWriter, r *http.Request) error {
	return h.startOrRestart(w, r, true)
}

func (h *Handler) startOrRestart(w http.ResponseWriter, r *http.Request, clearSession bool) error {
	id, appErr := parseWhatsappID(r)
	if appErr != nil {
		return appErr
	}
	loaded, appErr := h.Deps.Show(r.Context(), id)
	if appErr != nil {
		return appErr
	}

	updates := map[string]any{
		"status": StatusOpening,
		"qrcode": "",
	}
	if clearSession {
		updates["session"] = ""
	}
	if err := h.Deps.DB.WithContext(r.Context()).Model(&Whatsapp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}

	loaded.Status = StatusOpening
	loaded.QRCode = ""
	if clearSession {
		loaded.Session = ""
	}

	if err := PublishStartSession(r.Context(), h.Deps.RMQ, loaded); err != nil {
		h.logger().Warn("rmq publish session.start failed",
			slog.Uint64("whatsapp_id", uint64(loaded.ID)),
			slog.Any("err", err),
		)
		return errors.Wrap(err, errWorkerUnavailable, http.StatusServiceUnavailable)
	}

	if h.Deps.WS != nil {
		h.Deps.WS.Publish(wsChannelNotification, wsEventSessionUpdate, Serialize(loaded))
	}
	httpx.WriteJSON(w, http.StatusOK, messageResponse{Message: "Starting session."})
	return nil
}

func (h *Handler) sessionLogout(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseWhatsappID(r)
	if appErr != nil {
		return appErr
	}
	loaded, appErr := h.Deps.Show(r.Context(), id)
	if appErr != nil {
		return appErr
	}

	if err := PublishLogout(r.Context(), h.Deps.RMQ, loaded.ID); err != nil {
		h.logger().Warn("rmq publish session.logout failed",
			slog.Uint64("whatsapp_id", uint64(loaded.ID)),
			slog.Any("err", err),
		)
	}

	updates := map[string]any{
		"status":  StatusDisconnected,
		"qrcode":  "",
		"session": gorm.Expr("NULL"),
	}
	if err := h.Deps.DB.WithContext(r.Context()).Model(&Whatsapp{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errors.Wrap(err, "ERR_DB_UPDATE", http.StatusInternalServerError)
	}

	loaded.Status = StatusDisconnected
	loaded.QRCode = ""
	loaded.Session = ""

	if h.Deps.WS != nil {
		h.Deps.WS.Publish(wsChannelNotification, wsEventSessionUpdate, Serialize(loaded))
	}
	httpx.WriteJSON(w, http.StatusOK, messageResponse{Message: "Session disconnected."})
	return nil
}

func (h *Handler) sessionPairPhone(w http.ResponseWriter, r *http.Request) error {
	id, appErr := parseWhatsappID(r)
	if appErr != nil {
		return appErr
	}
	if _, appErr := h.Deps.Show(r.Context(), id); appErr != nil {
		return appErr
	}
	var body pairPhoneBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return errors.New("ERR_INVALID_INPUT", http.StatusBadRequest)
	}
	if body.Phone == "" {
		return errors.New("ERR_INVALID_INPUT", http.StatusBadRequest)
	}
	code, appErr := PairPhone(r.Context(), h.Deps.RPC, id, body.Phone)
	if appErr != nil {
		return appErr
	}
	httpx.WriteJSON(w, http.StatusOK, pairPhoneResponseBody{LinkingCode: code, QRCode: code})
	return nil
}

func parseWhatsappID(r *http.Request) (uint, *errors.AppError) {
	raw := chi.URLParam(r, "whatsappId")
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("ERR_BAD_REQUEST", http.StatusBadRequest)
	}
	return uint(parsed), nil
}
