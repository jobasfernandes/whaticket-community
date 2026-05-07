package rmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	exchangeWaEvents   = "wa.events"
	exchangeWaCommands = "wa.commands"
	exchangeWaRPC      = "wa.rpc"

	queueBackendWaEvents = "backend.wa-events"
	queueWorkerCommands  = "worker.commands"
	queueWorkerRPC       = "worker.rpc"

	bindingWaEventAll       = "wa.event.#"
	bindingSessionWildcard  = "session.*"
	bindingSessionPairPhone = "session.pairphone"
	bindingMatchAll         = "#"
)

func declareCommonExchanges(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(exchangeWaEvents, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rmq: declare %s: %w", exchangeWaEvents, err)
	}
	if err := ch.ExchangeDeclare(exchangeWaCommands, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rmq: declare %s: %w", exchangeWaCommands, err)
	}
	if err := ch.ExchangeDeclare(exchangeWaRPC, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rmq: declare %s: %w", exchangeWaRPC, err)
	}
	return nil
}

func declareBackendQueues(ch *amqp.Channel) error {
	if _, err := ch.QueueDeclare(queueBackendWaEvents, true, false, false, false, nil); err != nil {
		return fmt.Errorf("rmq: declare queue %s: %w", queueBackendWaEvents, err)
	}
	if err := ch.QueueBind(queueBackendWaEvents, bindingWaEventAll, exchangeWaEvents, false, nil); err != nil {
		return fmt.Errorf("rmq: bind %s -> %s: %w", queueBackendWaEvents, exchangeWaEvents, err)
	}
	return nil
}

func declareWorkerQueues(ch *amqp.Channel) error {
	if _, err := ch.QueueDeclare(queueWorkerCommands, true, false, false, false, nil); err != nil {
		return fmt.Errorf("rmq: declare queue %s: %w", queueWorkerCommands, err)
	}
	if err := ch.QueueBind(queueWorkerCommands, bindingSessionWildcard, exchangeWaCommands, false, nil); err != nil {
		return fmt.Errorf("rmq: bind %s -> %s (%s): %w", queueWorkerCommands, exchangeWaCommands, bindingSessionWildcard, err)
	}
	if err := ch.QueueBind(queueWorkerCommands, bindingSessionPairPhone, exchangeWaCommands, false, nil); err != nil {
		return fmt.Errorf("rmq: bind %s -> %s (%s): %w", queueWorkerCommands, exchangeWaCommands, bindingSessionPairPhone, err)
	}
	if _, err := ch.QueueDeclare(queueWorkerRPC, true, false, false, false, nil); err != nil {
		return fmt.Errorf("rmq: declare queue %s: %w", queueWorkerRPC, err)
	}
	if err := ch.QueueBind(queueWorkerRPC, bindingMatchAll, exchangeWaRPC, false, nil); err != nil {
		return fmt.Errorf("rmq: bind %s -> %s: %w", queueWorkerRPC, exchangeWaRPC, err)
	}
	return nil
}

func declareReplyQueue(ch *amqp.Channel) (string, error) {
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return "", fmt.Errorf("rmq: declare reply queue: %w", err)
	}
	return q.Name, nil
}
