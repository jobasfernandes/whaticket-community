package ws

import (
	"errors"
	"regexp"
	"strings"
)

const (
	ChannelSystem       = "system"
	ChannelNotification = "notification"
	ChannelGlobal       = "global"
	NamespaceTicket     = "ticket"
	NamespaceTickets    = "tickets"

	StatusPending = "pending"
	StatusOpen    = "open"
	StatusClosed  = "closed"

	CodeInvalidChannel = "ERR_INVALID_CHANNEL"
	CodeNoPermission   = "ERR_NO_PERMISSION"
	CodeInternal       = "ERR_INTERNAL"

	EventConnected = "connected"
	EventClosing   = "closing"
	EventError     = "error"
)

var (
	channelRegex   = regexp.MustCompile(`^[a-z_]+(:[a-zA-Z0-9_-]+)?$`)
	errInvalidName = errors.New("invalid channel name")
)

func ValidateChannelName(name string) error {
	if !channelRegex.MatchString(name) {
		return errInvalidName
	}
	return nil
}

func ParseChannel(name string) (namespace, suffix string, ok bool) {
	if err := ValidateChannelName(name); err != nil {
		return "", "", false
	}
	idx := strings.IndexByte(name, ':')
	if idx < 0 {
		return name, "", true
	}
	return name[:idx], name[idx+1:], true
}
