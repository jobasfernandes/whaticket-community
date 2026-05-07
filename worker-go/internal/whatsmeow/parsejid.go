package whatsmeow

import (
	"strings"

	"go.mau.fi/whatsmeow/types"
)

func ParseJID(arg string) (types.JID, bool) {
	return parseJID(arg)
}

func parseJID(arg string) (types.JID, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return types.JID{}, false
	}
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		if !isAllDigits(arg) {
			return types.JID{}, false
		}
		return types.NewJID(arg, types.DefaultUserServer), true
	}
	recipient, err := types.ParseJID(arg)
	if err != nil || recipient.User == "" {
		return recipient, false
	}
	switch recipient.Server {
	case types.DefaultUserServer, types.GroupServer, types.HiddenUserServer, types.BroadcastServer:
		return recipient, true
	default:
		return types.JID{}, false
	}
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
