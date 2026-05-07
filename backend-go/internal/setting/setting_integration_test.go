//go:build integration

package setting_test

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
	"github.com/jobasfernandes/whaticket-go-backend/internal/setting"
	"github.com/jobasfernandes/whaticket-go-backend/internal/wstest"
)

func TestUpdateUserCreationSetting(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	admin := dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", "secret-pass-123")

	wsRec := wstest.New()
	deps := &setting.Deps{DB: pg.DB, WS: wsRec}
	handler := &setting.Handler{Deps: deps}

	router := chi.NewRouter()
	handler.Routes(router, []byte(dbtest.TestAccessSecret))

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]string{"value": "disabled"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, srv.URL+"/settings/userCreation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /settings: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(raw))
	}

	var dto struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.Key != "userCreation" {
		t.Fatalf("unexpected key: %s", dto.Key)
	}

	var stored setting.Setting
	if err := pg.DB.WithContext(ctx).First(&stored, "key = ?", "userCreation").Error; err != nil {
		t.Fatalf("load setting: %v", err)
	}
	if stored.Value != "disabled" {
		t.Fatalf("expected value=disabled, got %s", stored.Value)
	}

	if events := wsRec.Find("settings.update"); len(events) == 0 {
		t.Fatal("expected settings.update WS event")
	}
}
