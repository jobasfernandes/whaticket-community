//go:build integration

package queue_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jobasfernandes/whaticket-go-backend/internal/db/dbtest"
	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
	"github.com/jobasfernandes/whaticket-go-backend/internal/wstest"
)

func TestCreateQueueAsAdmin(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	admin := dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", "secret-pass-123")

	wsRec := wstest.New()
	deps := &queue.Deps{DB: pg.DB, WS: wsRec}
	handler := &queue.Handler{Deps: deps}

	router := chi.NewRouter()
	handler.Routes(router, []byte(dbtest.TestAccessSecret))

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]any{
		"name":            "Sales",
		"color":           "#FF0000",
		"greetingMessage": "Hello",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/queue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /queue: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(raw))
	}

	var dto struct {
		ID    uint   `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ID == 0 || dto.Name != "Sales" {
		t.Fatalf("unexpected dto: %+v", dto)
	}

	var stored queue.Queue
	if err := pg.DB.WithContext(ctx).First(&stored, dto.ID).Error; err != nil {
		t.Fatalf("load queue: %v", err)
	}
	if stored.Color != "#FF0000" || stored.GreetingMessage != "Hello" {
		t.Fatalf("stored mismatch: %+v", stored)
	}

	if events := wsRec.Find("queue.created"); len(events) == 0 {
		t.Fatal("expected queue.created WS event")
	}
}
