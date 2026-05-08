//go:build integration

package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/canove/whaticket-community/backend/internal/db/dbtest"
	"github.com/canove/whaticket-community/backend/internal/setting"
	"github.com/canove/whaticket-community/backend/internal/user"
	"github.com/canove/whaticket-community/backend/internal/wstest"
)

func TestCreateUserAsAdmin(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	admin := dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", "secret-pass-123")

	wsRec := wstest.New()
	deps := &user.Deps{DB: pg.DB, WS: wsRec, Settings: setting.NewSettingChecker(pg.DB)}
	handler := &user.Handler{Deps: deps}

	router := chi.NewRouter()
	handler.Routes(router, []byte(dbtest.TestAccessSecret))

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]any{
		"name":     "New User",
		"email":    "newuser@example.com",
		"password": "another-pass-456",
		"profile":  "user",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(raw))
	}

	var dto struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ID == 0 || dto.Email != "newuser@example.com" || dto.Profile != "user" {
		t.Fatalf("unexpected dto: %+v", dto)
	}

	var stored user.User
	if err := pg.DB.WithContext(ctx).First(&stored, dto.ID).Error; err != nil {
		t.Fatalf("load user from DB: %v", err)
	}
	if stored.PasswordHash == "" {
		t.Fatal("password not hashed")
	}
	if stored.Email != "newuser@example.com" {
		t.Fatalf("stored email mismatch: %q", stored.Email)
	}

	events := wsRec.Find("user.created")
	if len(events) == 0 {
		t.Fatal("expected user.created WS event")
	}
}

func TestCreateUserUnauthorizedWithoutToken(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	deps := &user.Deps{DB: pg.DB, WS: wstest.New(), Settings: setting.NewSettingChecker(pg.DB)}
	handler := &user.Handler{Deps: deps}

	router := chi.NewRouter()
	handler.Routes(router, []byte(dbtest.TestAccessSecret))

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]string{"name": "x", "email": "x@x.com", "password": "abcdef"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
