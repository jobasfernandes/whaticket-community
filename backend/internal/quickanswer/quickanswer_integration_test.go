//go:build integration

package quickanswer_test

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
	"github.com/canove/whaticket-community/backend/internal/quickanswer"
	"github.com/canove/whaticket-community/backend/internal/wstest"
)

func TestCreateAndListQuickAnswer(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	admin := dbtest.SeedAdmin(t, pg, "Admin", "admin@example.com", "secret-pass-123")

	wsRec := wstest.New()
	deps := &quickanswer.Deps{DB: pg.DB, WS: wsRec}
	handler := &quickanswer.Handler{Deps: deps}

	router := chi.NewRouter()
	handler.Routes(router, []byte(dbtest.TestAccessSecret))

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]string{"shortcut": "/welcome", "message": "Welcome!"})
	postReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/quickAnswers", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Authorization", "Bearer "+admin.Token)
	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatalf("POST /quickAnswers: %v", err)
	}
	defer func() { _ = postResp.Body.Close() }()

	if postResp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(postResp.Body)
		t.Fatalf("post status=%d body=%s", postResp.StatusCode, string(raw))
	}

	var created struct {
		ID       uint   `json:"id"`
		Shortcut string `json:"shortcut"`
	}
	if err := json.NewDecoder(postResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode post: %v", err)
	}

	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/quickAnswers", nil)
	getReq.Header.Set("Authorization", "Bearer "+admin.Token)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET /quickAnswers: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()

	if getResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(getResp.Body)
		t.Fatalf("get status=%d body=%s", getResp.StatusCode, string(raw))
	}

	var list struct {
		QuickAnswers []struct {
			ID       uint   `json:"id"`
			Shortcut string `json:"shortcut"`
		} `json:"quickAnswers"`
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if list.Count == 0 {
		t.Fatal("expected at least 1 quick answer")
	}
	found := false
	for _, qa := range list.QuickAnswers {
		if qa.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created quick answer not in list (id=%d)", created.ID)
	}

	if events := wsRec.Find("quickAnswer.created"); len(events) == 0 {
		t.Fatal("expected quickAnswer.created WS event")
	}
}
