//go:build integration

package contact_test

import (
	"context"
	"testing"

	"github.com/canove/whaticket-community/backend/internal/contact"
	"github.com/canove/whaticket-community/backend/internal/db/dbtest"
	"github.com/canove/whaticket-community/backend/internal/wstest"
)

func TestCreateOrUpdateMergesByLID(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)

	wsRec := wstest.New()
	deps := &contact.Deps{DB: pg.DB, WS: wsRec}

	first, appErr := deps.CreateOrUpdate(ctx, contact.CreateOrUpdateRequest{
		Number: "+5511999998888",
		Name:   "Bob",
		LID:    "100200@lid",
	})
	if appErr != nil {
		t.Fatalf("first CreateOrUpdate: %v", appErr)
	}
	if first.ID == 0 {
		t.Fatal("expected first contact to have an ID")
	}
	if first.Number != "5511999998888" {
		t.Fatalf("expected normalized number, got %q", first.Number)
	}
	if first.Name != "Bob" {
		t.Fatalf("name=%q want Bob", first.Name)
	}

	second, appErr := deps.CreateOrUpdate(ctx, contact.CreateOrUpdateRequest{
		Number: "+5511999998888",
		Name:   "Bob Updated",
		LID:    "100200@lid",
	})
	if appErr != nil {
		t.Fatalf("second CreateOrUpdate: %v", appErr)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same contact id (%d), got %d", first.ID, second.ID)
	}
	if second.Name != "Bob Updated" {
		t.Fatalf("name=%q want Bob Updated", second.Name)
	}

	var count int64
	if err := pg.DB.WithContext(ctx).Model(&contact.Contact{}).Where("number = ?", "5511999998888").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}

	createdEvents := wsRec.Find("contact.created")
	updatedEvents := wsRec.Find("contact.updated")
	if len(createdEvents) != 1 {
		t.Fatalf("expected 1 contact.created event, got %d", len(createdEvents))
	}
	if len(updatedEvents) == 0 {
		t.Fatal("expected at least one contact.updated event")
	}
}
