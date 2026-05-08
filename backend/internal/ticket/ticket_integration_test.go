//go:build integration

package ticket_test

import (
	"context"
	"testing"

	"github.com/canove/whaticket-community/backend/internal/db/dbtest"
	"github.com/canove/whaticket-community/backend/internal/ticket"
	"github.com/canove/whaticket-community/backend/internal/wstest"
)

func TestFindOrCreateReusesPendingTicket(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	wsRec := wstest.New()
	wapp := dbtest.SeedWhatsapp(t, pg, "wpp-default")
	c := dbtest.SeedContact(t, pg, "Alice", "5511988887777")

	deps := &ticket.Deps{DB: pg.DB, WS: wsRec}

	first, appErr := deps.FindOrCreate(ctx, c, wapp.ID, 0, nil)
	if appErr != nil {
		t.Fatalf("first FindOrCreate: %v", appErr)
	}
	if first.ID == 0 {
		t.Fatal("expected ticket id")
	}
	if first.Status != "pending" {
		t.Fatalf("expected pending status, got %s", first.Status)
	}
	if first.ContactID != c.ID || first.WhatsappID != wapp.ID {
		t.Fatalf("ticket fields mismatch: %+v", first)
	}

	second, appErr := deps.FindOrCreate(ctx, c, wapp.ID, 3, nil)
	if appErr != nil {
		t.Fatalf("second FindOrCreate: %v", appErr)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same ticket id (got %d, want %d)", second.ID, first.ID)
	}
	if second.UnreadMessages != 3 {
		t.Fatalf("expected unread_messages=3, got %d", second.UnreadMessages)
	}

	var rows int64
	if err := pg.DB.WithContext(ctx).Model(&ticket.Ticket{}).
		Where("contact_id = ? AND whatsapp_id = ?", c.ID, wapp.ID).
		Count(&rows).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 ticket row, got %d", rows)
	}
}
