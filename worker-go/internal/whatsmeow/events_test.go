package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestEventTypeName(t *testing.T) {
	cases := []struct {
		evt  any
		want string
	}{
		{evt: &events.Connected{}, want: "Connected"},
		{evt: &events.Disconnected{}, want: "Disconnected"},
		{evt: &events.LoggedOut{}, want: "LoggedOut"},
		{evt: &events.Message{}, want: "Message"},
		{evt: &events.Receipt{}, want: "Receipt"},
		{evt: &events.HistorySync{}, want: "HistorySync"},
		{evt: &events.NewsletterMuteChange{}, want: "NewsletterMuteChange"},
	}
	for _, tc := range cases {
		got := eventTypeName(tc.evt)
		if got != tc.want {
			t.Errorf("eventTypeName(%T) = %q, want %q", tc.evt, got, tc.want)
		}
	}
}

func TestExtractBody(t *testing.T) {
	cases := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{
			name: "nil message",
			msg:  nil,
			want: "",
		},
		{
			name: "conversation",
			msg:  &waE2E.Message{Conversation: proto.String("hello")},
			want: "hello",
		},
		{
			name: "extended text",
			msg: &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text: proto.String("ext text"),
				},
			},
			want: "ext text",
		},
		{
			name: "image caption",
			msg: &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption: proto.String("img cap"),
				},
			},
			want: "img cap",
		},
		{
			name: "video caption",
			msg: &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					Caption: proto.String("vid cap"),
				},
			},
			want: "vid cap",
		},
		{
			name: "document caption",
			msg: &waE2E.Message{
				DocumentMessage: &waE2E.DocumentMessage{
					Caption: proto.String("doc cap"),
				},
			},
			want: "doc cap",
		},
		{
			name: "no body",
			msg:  &waE2E.Message{},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBody(tc.msg)
			if got != tc.want {
				t.Errorf("extractBody = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReceiptStateLabel(t *testing.T) {
	cases := []struct {
		rt   types.ReceiptType
		want string
	}{
		{rt: types.ReceiptTypeRead, want: "Read"},
		{rt: types.ReceiptTypeReadSelf, want: "ReadSelf"},
		{rt: types.ReceiptTypeDelivered, want: "Delivered"},
		{rt: types.ReceiptTypeSender, want: string(types.ReceiptTypeSender)},
	}
	for _, tc := range cases {
		got := receiptStateLabel(tc.rt)
		if got != tc.want {
			t.Errorf("receiptStateLabel(%v) = %q, want %q", tc.rt, got, tc.want)
		}
	}
}

func TestExtractBodyLRMGate(t *testing.T) {
	msg := &waE2E.Message{Conversation: proto.String(LRM + "spam")}
	body := extractBody(msg)
	if !HasLRMPrefix(body) {
		t.Fatalf("expected LRM prefix to be detected, body=%q", body)
	}

	plain := &waE2E.Message{Conversation: proto.String("normal")}
	if HasLRMPrefix(extractBody(plain)) {
		t.Fatal("plain message should not have LRM prefix")
	}
}
