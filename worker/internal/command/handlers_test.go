package command

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"go.mau.fi/whatsmeow"

	apperrors "github.com/canove/whaticket-community/worker/internal/platform/errors"
	"github.com/canove/whaticket-community/worker/internal/rmq"
)

func TestRouteCommandUnknownTypeReturnsNil(t *testing.T) {
	h := &Handlers{Log: testLogger()}
	env := rmq.Envelope{Type: "unknown.command"}
	if err := h.routeCommand(context.Background(), env); err != nil {
		t.Fatalf("expected nil for unknown command, got %v", err)
	}
}

func TestResolveJIDInvalid(t *testing.T) {
	if _, err := resolveJID(""); err == nil {
		t.Fatal("expected error for empty JID")
	}
	var appErr *apperrors.AppError
	_, err := resolveJID("not-a-jid!")
	if err == nil {
		t.Fatal("expected error for invalid JID")
	}
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if appErr.Code != apperrors.ErrInvalidPhone {
		t.Fatalf("want %s, got %s", apperrors.ErrInvalidPhone, appErr.Code)
	}
	if appErr.Status != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", appErr.Status)
	}
}

func TestResolveJIDValid(t *testing.T) {
	jid, err := resolveJID("+5511999998888")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jid.User != "5511999998888" {
		t.Fatalf("want user 5511999998888, got %q", jid.User)
	}
}

func TestPickMimeFallback(t *testing.T) {
	cases := []struct {
		explicit, detected, fallback, want string
	}{
		{"", "", "image/jpeg", "image/jpeg"},
		{"image/png", "image/jpeg", "image/jpeg", "image/png"},
		{"", "image/png", "image/jpeg", "image/png"},
		{"  ", "image/png", "image/jpeg", "image/png"},
	}
	for _, c := range cases {
		got := pickMime(c.explicit, c.detected, c.fallback)
		if got != c.want {
			t.Errorf("pickMime(%q,%q,%q) = %q, want %q", c.explicit, c.detected, c.fallback, got, c.want)
		}
	}
}

func TestMapPresence(t *testing.T) {
	cases := map[string]string{
		"available":   "available",
		"online":      "available",
		"unavailable": "unavailable",
		"offline":     "unavailable",
	}
	for input, want := range cases {
		got, err := mapPresence(input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", input, err)
			continue
		}
		if string(got) != want {
			t.Errorf("mapPresence(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := mapPresence("invalid"); err == nil {
		t.Error("expected error for invalid presence")
	}
}

func TestMapChatPresence(t *testing.T) {
	cases := []struct {
		input, state, media string
	}{
		{"composing", "composing", ""},
		{"typing", "composing", ""},
		{"recording", "composing", "audio"},
		{"paused", "paused", ""},
	}
	for _, c := range cases {
		state, m, err := mapChatPresence(c.input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", c.input, err)
			continue
		}
		if string(state) != c.state {
			t.Errorf("mapChatPresence(%q) state = %q, want %q", c.input, state, c.state)
		}
		if string(m) != c.media {
			t.Errorf("mapChatPresence(%q) media = %q, want %q", c.input, m, c.media)
		}
	}
	if _, _, err := mapChatPresence("nope"); err == nil {
		t.Error("expected error for invalid chat presence")
	}
}

func TestMapSendErrorClassification(t *testing.T) {
	cases := []struct {
		input    error
		wantCode string
		wantHTTP int
	}{
		{whatsmeow.ErrServerReturnedError, apperrors.ErrServerRejected, http.StatusBadGateway},
		{whatsmeow.ErrNotConnected, apperrors.ErrNoSession, http.StatusServiceUnavailable},
		{whatsmeow.ErrNotLoggedIn, apperrors.ErrNotLoggedIn, http.StatusBadRequest},
		{errors.New("random failure"), apperrors.ErrInternal, http.StatusInternalServerError},
	}
	for _, c := range cases {
		err := mapSendError(c.input)
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) {
			t.Errorf("mapSendError(%v) did not return *AppError", c.input)
			continue
		}
		if appErr.Code != c.wantCode {
			t.Errorf("mapSendError(%v).Code = %s, want %s", c.input, appErr.Code, c.wantCode)
		}
		if appErr.Status != c.wantHTTP {
			t.Errorf("mapSendError(%v).Status = %d, want %d", c.input, appErr.Status, c.wantHTTP)
		}
	}
}

func TestRequireLiveSessionNoSession(t *testing.T) {
	h := &Handlers{Log: testLogger()}
	if h.Mgr != nil {
		t.Fatal("expected nil manager")
	}
}

func TestBuildContextInfoNilOrEmpty(t *testing.T) {
	if got := buildContextInfo(nil); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
	if got := buildContextInfo(&ContextInfo{}); got != nil {
		t.Errorf("expected nil for empty stanzaID, got %+v", got)
	}
}

func TestBuildContextInfoFull(t *testing.T) {
	got := buildContextInfo(&ContextInfo{StanzaID: "abc", Participant: "5511@s.whatsapp.net"})
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.GetStanzaID() != "abc" {
		t.Errorf("StanzaID = %q, want %q", got.GetStanzaID(), "abc")
	}
	if got.GetParticipant() != "5511@s.whatsapp.net" {
		t.Errorf("Participant = %q, want %q", got.GetParticipant(), "5511@s.whatsapp.net")
	}
}
