package ws

import (
	"context"
	"strconv"
)

const profileAdmin = "admin"

type AuthzResult struct {
	Allowed bool
	Code    string
}

func authorizeSubscribe(ctx context.Context, ticketAuthz TicketAuthorizer, userID uint, profile string, channel string) AuthzResult {
	if err := ValidateChannelName(channel); err != nil {
		return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
	}
	namespace, suffix, ok := ParseChannel(channel)
	if !ok {
		return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
	}
	switch namespace {
	case ChannelSystem:
		if suffix != "" {
			return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
		}
		return AuthzResult{Allowed: false, Code: CodeNoPermission}
	case ChannelNotification, ChannelGlobal:
		if suffix != "" {
			return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
		}
		return AuthzResult{Allowed: true}
	case NamespaceTicket:
		if suffix == "" {
			return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
		}
		ticketID, err := strconv.ParseUint(suffix, 10, 64)
		if err != nil || ticketID == 0 {
			return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
		}
		if profile == profileAdmin {
			return AuthzResult{Allowed: true}
		}
		if ticketAuthz == nil {
			return AuthzResult{Allowed: false, Code: CodeInternal}
		}
		allowed, err := ticketAuthz.CanSee(ctx, userID, profile, uint(ticketID))
		if err != nil {
			return AuthzResult{Allowed: false, Code: CodeInternal}
		}
		if !allowed {
			return AuthzResult{Allowed: false, Code: CodeNoPermission}
		}
		return AuthzResult{Allowed: true}
	case NamespaceTickets:
		switch suffix {
		case StatusPending, StatusOpen, StatusClosed:
			return AuthzResult{Allowed: true}
		default:
			return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
		}
	default:
		return AuthzResult{Allowed: false, Code: CodeInvalidChannel}
	}
}
