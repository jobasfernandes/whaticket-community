package contact

import (
	"regexp"
	"strings"
)

var nonDigitRegex = regexp.MustCompile(`\D+`)

func NormalizeNumber(input string, isGroup bool) string {
	trimmed := strings.TrimSpace(input)
	if isGroup {
		return trimmed
	}
	return nonDigitRegex.ReplaceAllString(trimmed, "")
}
