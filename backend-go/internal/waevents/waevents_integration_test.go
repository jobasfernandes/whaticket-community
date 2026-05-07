//go:build integration

package waevents_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jobasfernandes/whaticket-go-backend/internal/contact"
	"github.com/jobasfernandes/whaticket-go-backend/internal/db/dbtest"
	"github.com/jobasfernandes/whaticket-go-backend/internal/message"
	apperr "github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
	"github.com/jobasfernandes/whaticket-go-backend/internal/rmq"
	"github.com/jobasfernandes/whaticket-go-backend/internal/rmqtest"
	"github.com/jobasfernandes/whaticket-go-backend/internal/ticket"
	"github.com/jobasfernandes/whaticket-go-backend/internal/waevents"
	"github.com/jobasfernandes/whaticket-go-backend/internal/whatsapp"
	"github.com/jobasfernandes/whaticket-go-backend/internal/wstest"
)

type whatsappAdapter struct {
	deps *whatsapp.Deps
}

func (a *whatsappAdapter) Show(ctx context.Context, id uint) (waevents.Whatsapp, error) {
	w, err := a.deps.Show(ctx, id)
	if err != nil {
		return nil, err
	}
	return &whatsappWrapper{w: w}, nil
}

func (a *whatsappAdapter) PublishStopSession(_ context.Context, _ uint) error { return nil }
func (a *whatsappAdapter) PublishLogout(_ context.Context, _ uint) error      { return nil }
func (a *whatsappAdapter) UpdateStatus(ctx context.Context, id uint, status, qrcode string) error {
	if err := a.deps.UpdateStatus(ctx, id, status, qrcode); err != nil {
		return err
	}
	return nil
}
func (a *whatsappAdapter) UpdateConnected(ctx context.Context, id uint) error {
	if err := a.deps.UpdateConnected(ctx, id); err != nil {
		return err
	}
	return nil
}
func (a *whatsappAdapter) UpdateRetries(ctx context.Context, id uint) error {
	if err := a.deps.UpdateRetries(ctx, id); err != nil {
		return err
	}
	return nil
}
func (a *whatsappAdapter) UpdateDisconnected(ctx context.Context, id uint) error {
	if err := a.deps.UpdateDisconnected(ctx, id); err != nil {
		return err
	}
	return nil
}
func (a *whatsappAdapter) SerializeForWS(w waevents.Whatsapp) any {
	if wp, ok := w.(*whatsappWrapper); ok && wp.w != nil {
		return whatsapp.Serialize(wp.w)
	}
	return nil
}

type whatsappWrapper struct {
	w *whatsapp.Whatsapp
}

func (wp *whatsappWrapper) GetID() uint                { return wp.w.ID }
func (wp *whatsappWrapper) GetStatus() string          { return wp.w.Status }
func (wp *whatsappWrapper) GetGreetingMessage() string { return wp.w.GreetingMessage }
func (wp *whatsappWrapper) GetFarewellMessage() string { return wp.w.FarewellMessage }
func (wp *whatsappWrapper) GetQueues() []waevents.QueueLike {
	out := make([]waevents.QueueLike, 0, len(wp.w.Queues))
	for i := range wp.w.Queues {
		out = append(out, &queueWrapper{q: &wp.w.Queues[i]})
	}
	return out
}

type queueWrapper struct{ q *queue.Queue }

func (qw *queueWrapper) GetID() uint                { return qw.q.ID }
func (qw *queueWrapper) GetName() string            { return qw.q.Name }
func (qw *queueWrapper) GetGreetingMessage() string { return qw.q.GreetingMessage }

type contactAdapter struct{ deps *contact.Deps }

func (a *contactAdapter) CreateOrUpdate(ctx context.Context, number, name, lid string, isGroup bool) (waevents.Contact, error) {
	c, err := a.deps.CreateOrUpdate(ctx, contact.CreateOrUpdateRequest{
		Number: number, Name: name, LID: lid, IsGroup: isGroup,
	})
	if err != nil {
		return nil, err
	}
	return &contactWrapper{c: c}, nil
}

func (a *contactAdapter) Create(_ context.Context, _, _ string) error { return nil }

type contactWrapper struct{ c *contact.Contact }

func (cw *contactWrapper) GetID() uint       { return cw.c.ID }
func (cw *contactWrapper) GetNumber() string { return cw.c.Number }
func (cw *contactWrapper) GetName() string   { return cw.c.Name }

type ticketAdapter struct{ deps *ticket.Deps }

func (a *ticketAdapter) FindOrCreate(_ context.Context, _ waevents.Contact, _ uint, _ int, _ waevents.Contact) (waevents.Ticket, error) {
	return nil, nil
}
func (a *ticketAdapter) UpdateLastMessage(_ context.Context, _ uint, _ string) error { return nil }
func (a *ticketAdapter) UpdateQueue(_ context.Context, _, _ uint) error              { return nil }
func (a *ticketAdapter) SetMessagesAsRead(_ context.Context, _ uint) error           { return nil }

type messageAdapter struct{ deps *message.Deps }

func (a *messageAdapter) Create(_ context.Context, _ waevents.MessageData) error { return nil }

func (a *messageAdapter) BuildAckUpdatePayload(_ context.Context, _ string) (any, bool) {
	return nil, false
}

type rmqEnvelopeAdapter struct{ client *rmq.Client }

func (a *rmqEnvelopeAdapter) Publish(ctx context.Context, exchange, routingKey string, env any) error {
	switch v := env.(type) {
	case rmq.Envelope:
		return a.client.Publish(ctx, exchange, routingKey, v)
	default:
		return fmt.Errorf("unsupported envelope type %T", env)
	}
}

func TestQRCodeEventUpdatesWhatsappRow(t *testing.T) {
	ctx := context.Background()
	pg := dbtest.StartPostgres(ctx, t)
	rmqEnv := rmqtest.StartRabbitMQ(ctx, t)

	wapp := dbtest.SeedWhatsapp(t, pg, "wpp-default")

	wsRec := wstest.New()
	whatsappDeps := &whatsapp.Deps{DB: pg.DB, WS: wsRec, RMQ: rmqEnv.Client, RPC: rmqEnv.Client}
	contactDeps := &contact.Deps{DB: pg.DB, WS: wsRec}
	ticketDeps := &ticket.Deps{DB: pg.DB, WS: wsRec}
	messageDeps := &message.Deps{DB: pg.DB, WS: wsRec}

	consumer := &waevents.Consumer{
		DB:          pg.DB,
		RMQ:         &rmqEnvelopeAdapter{client: rmqEnv.Client},
		RPC:         rmqEnv.Client,
		WS:          wsRec,
		WhatsappSvc: &whatsappAdapter{deps: whatsappDeps},
		ContactSvc:  &contactAdapter{deps: contactDeps},
		TicketSvc:   &ticketAdapter{deps: ticketDeps},
		MessageSvc:  &messageAdapter{deps: messageDeps},
	}

	consumeCtx, cancelConsume := context.WithCancel(ctx)
	t.Cleanup(cancelConsume)

	go func() {
		_ = consumer.Start(consumeCtx, rmqEnv.Client)
	}()

	time.Sleep(500 * time.Millisecond)

	payload := struct {
		Code string `json:"code"`
	}{Code: "fake-qr-code-payload"}

	routingKey := fmt.Sprintf("wa.event.%d.qr.code", wapp.ID)
	envelope, err := rmq.WrapPayload(routingKey, int(wapp.ID), payload)
	if err != nil {
		t.Fatalf("wrap payload: %v", err)
	}
	if err := rmqEnv.Client.Publish(ctx, "wa.events", routingKey, envelope); err != nil {
		t.Fatalf("publish wa.events: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var stored whatsapp.Whatsapp
		if err := pg.DB.WithContext(ctx).First(&stored, wapp.ID).Error; err != nil {
			t.Fatalf("load whatsapp: %v", err)
		}
		if stored.QRCode == "fake-qr-code-payload" && stored.Status == "qrcode" {
			updates := wsRec.Find("whatsappSession.update")
			if len(updates) == 0 {
				t.Fatal("expected whatsappSession.update WS event")
			}
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("timed out waiting for QR code to be persisted")
}

var _ = []any{(*apperr.AppError)(nil)}
