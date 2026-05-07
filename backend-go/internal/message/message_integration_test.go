//go:build integration

package message_test

import (
	"context"
	"testing"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/db/dbtest"
	"github.com/jobasfernandes/whaticket-go-backend/internal/message"
	"github.com/jobasfernandes/whaticket-go-backend/internal/ticket"
	"github.com/jobasfernandes/whaticket-go-backend/internal/wstest"

	apperr "github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
)

type stubTicketSvc struct {
	updates int
}

func (s *stubTicketSvc) Show(_ context.Context, _ uint, _ *auth.UserClaims) (message.TicketLike, *apperr.AppError) {
	return nil, nil
}

func (s *stubTicketSvc) LoadByID(_ context.Context, _ uint) (message.TicketLike, *apperr.AppError) {
	return nil, nil
}

func (s *stubTicketSvc) UpdateLastMessage(_ context.Context, _ uint, _ string) *apperr.AppError {
	s.updates++
	return nil
}

func (s *stubTicketSvc) SerializeTicket(_ message.TicketLike) any { return nil }

func TestCreateMessageUpsertsAndKeepsHigherAck(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	wapp := dbtest.SeedWhatsapp(t, pg, "wpp-default")
	c := dbtest.SeedContact(t, pg, "Alice", "5511988887777")

	tk := &ticket.Ticket{
		Status:     "pending",
		ContactID:  c.ID,
		WhatsappID: wapp.ID,
	}
	if err := pg.DB.WithContext(ctx).Create(tk).Error; err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	wsRec := wstest.New()
	tsvc := &stubTicketSvc{}
	deps := &message.Deps{DB: pg.DB, WS: wsRec, TicketSvc: tsvc}

	contactID := c.ID
	first, appErr := deps.Create(ctx, message.MessageData{
		ID:        "MSG-001",
		TicketID:  tk.ID,
		ContactID: &contactID,
		Body:      "hello",
		FromMe:    false,
		Ack:       2,
	})
	if appErr != nil {
		t.Fatalf("first Create: %v", appErr)
	}
	if first.ID != "MSG-001" || first.Ack != 2 || first.Body != "hello" {
		t.Fatalf("unexpected first message: %+v", first)
	}

	second, appErr := deps.Create(ctx, message.MessageData{
		ID:        "MSG-001",
		TicketID:  tk.ID,
		ContactID: &contactID,
		Body:      "hello again",
		FromMe:    false,
		Ack:       1,
	})
	if appErr != nil {
		t.Fatalf("second Create: %v", appErr)
	}
	if second.Ack != 2 {
		t.Fatalf("expected ack to remain 2 (not downgrade), got %d", second.Ack)
	}
	if second.Body != "hello again" {
		t.Fatalf("expected body update, got %q", second.Body)
	}

	var count int64
	if err := pg.DB.WithContext(ctx).Model(&message.Message{}).
		Where("ticket_id = ?", tk.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message row, got %d", count)
	}

	if events := wsRec.Find("appMessage.create"); len(events) != 4 {
		t.Fatalf("expected 4 appMessage.create events (2 calls × ticket+notification channels), got %d", len(events))
	}
}
