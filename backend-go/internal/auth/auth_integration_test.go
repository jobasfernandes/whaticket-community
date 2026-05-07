//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/db/dbtest"
	"github.com/jobasfernandes/whaticket-go-backend/internal/user"
)

func TestLoginHappyPath(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	const password = "secret-pass-123"
	seeded := dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", password)

	router := chi.NewRouter()
	authDeps := &auth.Deps{
		DB:            pg.DB,
		Loader:        user.NewAuthLoader(pg.DB),
		AccessSecret:  []byte(dbtest.TestAccessSecret),
		RefreshSecret: []byte(dbtest.TestRefreshSecret),
	}
	authDeps.Routes(router)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]string{"email": seeded.User.Email, "password": password})
	resp, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(raw))
	}

	var session struct {
		Token string `json:"token"`
		User  struct {
			ID    uint   `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if session.Token == "" {
		t.Fatal("expected non-empty access token")
	}
	if session.User.ID != seeded.User.ID || session.User.Email != seeded.User.Email {
		t.Fatalf("unexpected user payload: %+v", session.User)
	}

	cookies := resp.Cookies()
	var jrt *http.Cookie
	for _, c := range cookies {
		if c.Name == "jrt" {
			jrt = c
			break
		}
	}
	if jrt == nil {
		t.Fatal("expected jrt refresh cookie")
	}
	if jrt.Value == "" {
		t.Fatal("expected non-empty jrt cookie value")
	}
	if !jrt.HttpOnly {
		t.Error("jrt cookie should be HttpOnly")
	}

	claims, err := auth.ParseAccessToken(session.Token, []byte(dbtest.TestAccessSecret))
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.ID != seeded.User.ID {
		t.Fatalf("token id=%d want %d", claims.ID, seeded.User.ID)
	}
	if claims.Profile != "admin" {
		t.Fatalf("token profile=%s want admin", claims.Profile)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	_ = dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", "right-pass")

	router := chi.NewRouter()
	authDeps := &auth.Deps{
		DB:            pg.DB,
		Loader:        user.NewAuthLoader(pg.DB),
		AccessSecret:  []byte(dbtest.TestAccessSecret),
		RefreshSecret: []byte(dbtest.TestRefreshSecret),
	}
	authDeps.Routes(router)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]string{"email": "admin@example.com", "password": "wrong-pass"})
	resp, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
