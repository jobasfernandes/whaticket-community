//go:build integration

package whatsapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/canove/whaticket-community/backend/internal/db/dbtest"
	"github.com/canove/whaticket-community/backend/internal/rmq"
	"github.com/canove/whaticket-community/backend/internal/rmqtest"
	"github.com/canove/whaticket-community/backend/internal/whatsapp"
	"github.com/canove/whaticket-community/backend/internal/wstest"
)

func TestCreateWhatsappPublishesSessionStart(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)
	rmqEnv := rmqtest.StartRabbitMQAsWorker(ctx, t)

	admin := dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", "secret-pass-123")

	wsRec := wstest.New()

	consumeCtx, cancelConsume := context.WithCancel(ctx)
	t.Cleanup(cancelConsume)

	envCh := make(chan rmq.Envelope, 4)
	go func() {
		_ = rmqEnv.Client.Consume(consumeCtx, "worker.commands", "test-cmd-consumer", func(_ context.Context, env rmq.Envelope) error {
			select {
			case envCh <- env:
			default:
			}
			return nil
		})
	}()

	deps := &whatsapp.Deps{
		DB:  pg.DB,
		WS:  wsRec,
		RMQ: rmqEnv.Client,
		RPC: rmqEnv.Client,
	}
	handler := &whatsapp.Handler{Deps: deps, AccessSecret: []byte(dbtest.TestAccessSecret)}

	router := chi.NewRouter()
	handler.Routes(router)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]any{
		"name":            "wpp-1",
		"greetingMessage": "",
		"farewellMessage": "",
		"isDefault":       true,
		"mediaDelivery":   "base64",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /whatsapp: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(raw))
	}

	var dto struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ID == 0 || dto.Name != "wpp-1" {
		t.Fatalf("unexpected dto: %+v", dto)
	}

	var stored whatsapp.Whatsapp
	if err := pg.DB.WithContext(ctx).First(&stored, dto.ID).Error; err != nil {
		t.Fatalf("load whatsapp: %v", err)
	}
	if !stored.IsDefault {
		t.Fatal("expected stored is_default=true")
	}

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	select {
	case env := <-envCh:
		if env.Type != "session.start" {
			t.Fatalf("expected session.start envelope, got %s", env.Type)
		}
		if env.UserID != int(dto.ID) {
			t.Fatalf("expected user id %d, got %d", dto.ID, env.UserID)
		}
	case <-deadline.C:
		t.Fatal("timed out waiting for session.start envelope on worker.commands")
	}
}
