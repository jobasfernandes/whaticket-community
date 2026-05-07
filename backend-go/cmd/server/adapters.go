package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jobasfernandes/whaticket-go-backend/internal/auth"
	"github.com/jobasfernandes/whaticket-go-backend/internal/contact"
	"github.com/jobasfernandes/whaticket-go-backend/internal/message"
	apperr "github.com/jobasfernandes/whaticket-go-backend/internal/platform/errors"
	"github.com/jobasfernandes/whaticket-go-backend/internal/queue"
	"github.com/jobasfernandes/whaticket-go-backend/internal/rmq"
	"github.com/jobasfernandes/whaticket-go-backend/internal/ticket"
	"github.com/jobasfernandes/whaticket-go-backend/internal/waevents"
	"github.com/jobasfernandes/whaticket-go-backend/internal/whatsapp"
)

type messageTicketAdapter struct {
	deps *ticket.Deps
}

func newMessageTicketAdapter(deps *ticket.Deps) *messageTicketAdapter {
	return &messageTicketAdapter{deps: deps}
}

func (a *messageTicketAdapter) Show(ctx context.Context, ticketID uint, actor *auth.UserClaims) (message.TicketLike, *apperr.AppError) {
	t, err := a.deps.Show(ctx, ticketID, actor)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (a *messageTicketAdapter) UpdateLastMessage(ctx context.Context, ticketID uint, body string) *apperr.AppError {
	return a.deps.UpdateLastMessage(ctx, ticketID, body)
}

func (a *messageTicketAdapter) SerializeTicket(t message.TicketLike) any {
	if t == nil {
		return nil
	}
	if real, ok := t.(*ticket.Ticket); ok {
		return ticket.Serialize(real)
	}
	return t
}

type rmqEnvelopeAdapter struct {
	client *rmq.Client
}

func newRMQEnvelopeAdapter(c *rmq.Client) *rmqEnvelopeAdapter {
	return &rmqEnvelopeAdapter{client: c}
}

func (a *rmqEnvelopeAdapter) Publish(ctx context.Context, exchange, routingKey string, env any) error {
	switch v := env.(type) {
	case rmq.Envelope:
		return a.client.Publish(ctx, exchange, routingKey, v)
	case *rmq.Envelope:
		if v == nil {
			return fmt.Errorf("rmq envelope adapter: nil envelope")
		}
		return a.client.Publish(ctx, exchange, routingKey, *v)
	default:
		raw, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("rmq envelope adapter: marshal: %w", err)
		}
		envelope := rmq.Envelope{
			Type:    routingKey,
			Payload: raw,
		}
		return a.client.Publish(ctx, exchange, routingKey, envelope)
	}
}

type waeventsWhatsappAdapter struct {
	deps *whatsapp.Deps
	rpc  *rmq.Client
}

func newWaeventsWhatsappAdapter(deps *whatsapp.Deps, rpc *rmq.Client) *waeventsWhatsappAdapter {
	return &waeventsWhatsappAdapter{deps: deps, rpc: rpc}
}

func (a *waeventsWhatsappAdapter) Show(ctx context.Context, id uint) (waevents.Whatsapp, error) {
	w, appErr := a.deps.Show(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	return &whatsappWrapper{w: w}, nil
}

func (a *waeventsWhatsappAdapter) PublishStopSession(ctx context.Context, whatsappID uint) error {
	return whatsapp.PublishStopSession(ctx, a.deps.RMQ, whatsappID)
}

func (a *waeventsWhatsappAdapter) PublishLogout(ctx context.Context, whatsappID uint) error {
	return whatsapp.PublishLogout(ctx, a.deps.RMQ, whatsappID)
}

func (a *waeventsWhatsappAdapter) UpdateStatus(ctx context.Context, id uint, status, qrcode string) error {
	if appErr := a.deps.UpdateStatus(ctx, id, status, qrcode); appErr != nil {
		return appErr
	}
	return nil
}

func (a *waeventsWhatsappAdapter) UpdateConnected(ctx context.Context, id uint) error {
	if appErr := a.deps.UpdateConnected(ctx, id); appErr != nil {
		return appErr
	}
	return nil
}

func (a *waeventsWhatsappAdapter) UpdateRetries(ctx context.Context, id uint) error {
	if appErr := a.deps.UpdateRetries(ctx, id); appErr != nil {
		return appErr
	}
	return nil
}

func (a *waeventsWhatsappAdapter) UpdateDisconnected(ctx context.Context, id uint) error {
	if appErr := a.deps.UpdateDisconnected(ctx, id); appErr != nil {
		return appErr
	}
	return nil
}

func (a *waeventsWhatsappAdapter) SerializeForWS(w waevents.Whatsapp) any {
	if w == nil {
		return nil
	}
	wrapper, ok := w.(*whatsappWrapper)
	if !ok || wrapper.w == nil {
		return nil
	}
	return whatsapp.Serialize(wrapper.w)
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

type queueWrapper struct {
	q *queue.Queue
}

func (qw *queueWrapper) GetID() uint                { return qw.q.ID }
func (qw *queueWrapper) GetName() string            { return qw.q.Name }
func (qw *queueWrapper) GetGreetingMessage() string { return qw.q.GreetingMessage }

type waeventsContactAdapter struct {
	deps *contact.Deps
}

func newWaeventsContactAdapter(deps *contact.Deps) *waeventsContactAdapter {
	return &waeventsContactAdapter{deps: deps}
}

func (a *waeventsContactAdapter) CreateOrUpdate(ctx context.Context, number, name, lid string, isGroup bool) (waevents.Contact, error) {
	req := contact.CreateOrUpdateRequest{
		Number:  number,
		Name:    name,
		LID:     lid,
		IsGroup: isGroup,
	}
	c, appErr := a.deps.CreateOrUpdate(ctx, req)
	if appErr != nil {
		return nil, appErr
	}
	return &contactWrapper{c: c}, nil
}

func (a *waeventsContactAdapter) Create(ctx context.Context, name, number string) error {
	req := contact.CreateRequest{Name: name, Number: number}
	if _, appErr := a.deps.Create(ctx, req); appErr != nil {
		return appErr
	}
	return nil
}

type contactWrapper struct {
	c *contact.Contact
}

func (cw *contactWrapper) GetID() uint       { return cw.c.ID }
func (cw *contactWrapper) GetNumber() string { return cw.c.Number }
func (cw *contactWrapper) GetName() string {
	if cw.c.Name != "" {
		return cw.c.Name
	}
	return cw.c.Number
}

type waeventsTicketAdapter struct {
	deps *ticket.Deps
}

func newWaeventsTicketAdapter(deps *ticket.Deps) *waeventsTicketAdapter {
	return &waeventsTicketAdapter{deps: deps}
}

func (a *waeventsTicketAdapter) FindOrCreate(ctx context.Context, c waevents.Contact, whatsappID uint, unreadMessages int, groupContact waevents.Contact) (waevents.Ticket, error) {
	contactRef := unwrapContact(c)
	groupRef := unwrapContact(groupContact)
	t, appErr := a.deps.FindOrCreate(ctx, contactRef, whatsappID, unreadMessages, groupRef)
	if appErr != nil {
		return nil, appErr
	}
	return t, nil
}

func (a *waeventsTicketAdapter) UpdateLastMessage(ctx context.Context, ticketID uint, body string) error {
	if appErr := a.deps.UpdateLastMessage(ctx, ticketID, body); appErr != nil {
		return appErr
	}
	return nil
}

func (a *waeventsTicketAdapter) UpdateQueue(ctx context.Context, ticketID uint, queueID uint) error {
	if appErr := a.deps.UpdateQueue(ctx, ticketID, queueID); appErr != nil {
		return appErr
	}
	return nil
}

func (a *waeventsTicketAdapter) SetMessagesAsRead(ctx context.Context, ticketID uint) error {
	if appErr := a.deps.SetMessagesAsRead(ctx, ticketID); appErr != nil {
		return appErr
	}
	return nil
}

func unwrapContact(c waevents.Contact) *contact.Contact {
	if c == nil {
		return nil
	}
	if w, ok := c.(*contactWrapper); ok {
		return w.c
	}
	return &contact.Contact{
		ID:      c.GetID(),
		Number:  c.GetNumber(),
		Name:    c.GetName(),
		IsGroup: strings.HasSuffix(c.GetNumber(), "g.us"),
	}
}

type waeventsMessageAdapter struct {
	deps *message.Deps
}

func newWaeventsMessageAdapter(deps *message.Deps) *waeventsMessageAdapter {
	return &waeventsMessageAdapter{deps: deps}
}

func (a *waeventsMessageAdapter) Create(ctx context.Context, data waevents.MessageData) error {
	msg := message.MessageData{
		ID:          data.ID,
		TicketID:    data.TicketID,
		ContactID:   data.ContactID,
		Body:        data.Body,
		FromMe:      data.FromMe,
		Read:        data.Read,
		MediaType:   data.MediaType,
		MediaURL:    data.MediaURL,
		QuotedMsgID: data.QuotedMsgID,
		Ack:         data.Ack,
	}
	if _, appErr := a.deps.Create(ctx, msg); appErr != nil {
		return appErr
	}
	return nil
}
