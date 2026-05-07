package rmq

import "errors"

var (
	ErrNotConnected          = errors.New("rmq: not connected")
	ErrDisabled              = errors.New("rmq: disabled after exhausted reconnects")
	ErrShuttingDown          = errors.New("rmq: shutting down")
	ErrPublishConfirmTimeout = errors.New("rmq: publish confirm timeout")
	ErrInvalidEnvelope       = errors.New("rmq: invalid envelope")
	ErrConnectionLost        = errors.New("rmq: connection lost")
)
