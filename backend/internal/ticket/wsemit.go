package ticket

import "fmt"

type WSPublisher interface {
	Publish(channel, event string, data any)
}

const (
	wsEventTicketUpdate = "ticket.update"
	wsEventTicketDelete = "ticket.delete"
	wsChannelNotify     = "notification"
	wsActionCreate      = "create"
	wsActionUpdate      = "update"
	wsActionDelete      = "delete"
)

func emitCreate(ws WSPublisher, dto TicketDTO) {
	if ws == nil {
		return
	}
	payload := map[string]any{"action": wsActionCreate, "ticket": dto}
	ws.Publish(ticketStatusChannel(dto.Status), wsEventTicketUpdate, payload)
	ws.Publish(wsChannelNotify, wsEventTicketUpdate, payload)
}

func emitUpdate(ws WSPublisher, dto TicketDTO, oldStatus string) {
	if ws == nil {
		return
	}
	payload := map[string]any{"action": wsActionUpdate, "ticket": dto}
	if oldStatus != dto.Status {
		ws.Publish(ticketStatusChannel(oldStatus), wsEventTicketUpdate, payload)
		ws.Publish(ticketStatusChannel(dto.Status), wsEventTicketUpdate, payload)
		ws.Publish(ticketChannel(dto.ID), wsEventTicketUpdate, payload)
		ws.Publish(wsChannelNotify, wsEventTicketUpdate, payload)
		return
	}
	ws.Publish(ticketStatusChannel(dto.Status), wsEventTicketUpdate, payload)
	ws.Publish(ticketChannel(dto.ID), wsEventTicketUpdate, payload)
}

func emitDelete(ws WSPublisher, dto TicketDTO) {
	if ws == nil {
		return
	}
	payload := map[string]any{"action": wsActionDelete, "ticketId": dto.ID}
	ws.Publish(ticketStatusChannel(dto.Status), wsEventTicketDelete, payload)
	ws.Publish(ticketChannel(dto.ID), wsEventTicketDelete, payload)
	ws.Publish(wsChannelNotify, wsEventTicketDelete, payload)
}

func ticketStatusChannel(status string) string {
	return "tickets:" + status
}

func ticketChannel(id uint) string {
	return fmt.Sprintf("ticket:%d", id)
}
