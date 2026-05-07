package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestParseJID(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantOK   bool
		wantUser string
		wantSrv  string
	}{
		{name: "empty", input: "", wantOK: false},
		{name: "whitespace only", input: "   ", wantOK: false},
		{name: "plus prefix digits", input: "+5511999999999", wantOK: true, wantUser: "5511999999999", wantSrv: types.DefaultUserServer},
		{name: "raw digits", input: "5511999999999", wantOK: true, wantUser: "5511999999999", wantSrv: types.DefaultUserServer},
		{name: "letters without server", input: "abc", wantOK: false},
		{name: "user server explicit", input: "5511999999999@s.whatsapp.net", wantOK: true, wantUser: "5511999999999", wantSrv: types.DefaultUserServer},
		{name: "group server", input: "120363012345678901@g.us", wantOK: true, wantSrv: types.GroupServer},
		{name: "lid server", input: "1234567@lid", wantOK: true, wantSrv: types.HiddenUserServer},
		{name: "broadcast server", input: "abc@broadcast", wantOK: true, wantSrv: types.BroadcastServer},
		{name: "unknown server", input: "5511999999999@example.com", wantOK: false},
		{name: "malformed at", input: "@@@@", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jid, ok := parseJID(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("parseJID(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if tc.wantUser != "" && jid.User != tc.wantUser {
				t.Errorf("parseJID(%q) user = %q, want %q", tc.input, jid.User, tc.wantUser)
			}
			if tc.wantSrv != "" && jid.Server != tc.wantSrv {
				t.Errorf("parseJID(%q) server = %q, want %q", tc.input, jid.Server, tc.wantSrv)
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	cases := map[string]bool{
		"":      true,
		"123":   true,
		"55119": true,
		"abc":   false,
		"12a":   false,
		"+551":  false,
		"  9 ":  false,
	}
	for input, want := range cases {
		if got := isAllDigits(input); got != want {
			t.Errorf("isAllDigits(%q) = %v, want %v", input, got, want)
		}
	}
}
