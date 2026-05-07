package whatsmeow

import "testing"

func TestHasLRMPrefix(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{body: "", want: false},
		{body: "hello", want: false},
		{body: LRM + "hello", want: true},
		{body: "hello" + LRM, want: false},
		{body: "  " + LRM + "hello", want: false},
	}
	for _, tc := range cases {
		if got := HasLRMPrefix(tc.body); got != tc.want {
			t.Errorf("HasLRMPrefix(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}
