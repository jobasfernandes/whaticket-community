package whatsmeow

import "strings"

const LRM = "\u200e"

func HasLRMPrefix(body string) bool {
	return strings.HasPrefix(body, LRM)
}
