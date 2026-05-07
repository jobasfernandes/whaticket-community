package rmq

import (
	"encoding/json"
	"fmt"
	"time"
)

const envelopeVersion = "1"

type Envelope struct {
	Version   string          `json:"v"`
	Timestamp int64           `json:"ts"`
	Type      string          `json:"type"`
	UserID    int             `json:"userId"`
	Payload   json.RawMessage `json:"payload"`
	Error     *EnvelopeError  `json:"error,omitempty"`
}

type EnvelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status,omitempty"`
}

func WrapPayload(eventType string, userID int, payload any) (Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		bytes, err := json.Marshal(payload)
		if err != nil {
			return Envelope{}, fmt.Errorf("rmq: marshal payload: %w", err)
		}
		raw = bytes
	} else {
		raw = json.RawMessage("null")
	}
	return Envelope{
		Version:   envelopeVersion,
		Timestamp: time.Now().UnixMilli(),
		Type:      eventType,
		UserID:    userID,
		Payload:   raw,
	}, nil
}

func (e Envelope) Decode(into any) error {
	if into == nil {
		return fmt.Errorf("rmq: decode target is nil")
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("rmq: empty payload")
	}
	if err := json.Unmarshal(e.Payload, into); err != nil {
		return fmt.Errorf("rmq: decode payload: %w", err)
	}
	return nil
}
