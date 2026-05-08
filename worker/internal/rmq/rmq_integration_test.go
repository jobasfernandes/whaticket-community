//go:build integration

package rmq_test

import (
	"context"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/canove/whaticket-community/worker/internal/rmq"
	"github.com/canove/whaticket-community/worker/internal/testenv"
)

const (
	testExchange   = "wa.events"
	testRoutingKey = "wa.event.42.TestPing"
	testQueue      = "rmq-test-publish-consume"
)

type pingPayload struct {
	Hello string `json:"hello"`
	N     int    `json:"n"`
}

type rpcRequest struct {
	Q string `json:"q"`
}

type rpcResponse struct {
	A string `json:"a"`
}

func TestPublishAndConsumeRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	env := testenv.StartRabbitMQ(ctx, t)
	bindTestQueue(t, env.URL)

	consumeCtx, consumeCancel := context.WithCancel(ctx)
	defer consumeCancel()

	received := make(chan rmq.Envelope, 1)
	go func() {
		_ = env.Client.Consume(consumeCtx, testQueue, "rmq-test-consumer", func(_ context.Context, e rmq.Envelope) error {
			select {
			case received <- e:
			default:
			}
			return nil
		})
	}()

	envelope, err := rmq.WrapPayload("TestPing", 7, pingPayload{Hello: "world", N: 42})
	if err != nil {
		t.Fatalf("wrap payload: %v", err)
	}

	pubCtx, pubCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pubCancel()
	if err := env.Client.Publish(pubCtx, testExchange, testRoutingKey, envelope); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got.Type != "TestPing" {
			t.Fatalf("envelope type=%q want TestPing", got.Type)
		}
		var payload pingPayload
		if err := got.Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Hello != "world" || payload.N != 42 {
			t.Fatalf("payload mismatch: %+v", payload)
		}
		if got.UserID != 7 {
			t.Fatalf("envelope userId=%d want 7", got.UserID)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("did not receive published envelope within 15s")
	}
}

func TestRPCRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	env := testenv.StartRabbitMQ(ctx, t)

	const rpcType = "test.rpc.echo"
	bindRPCRoutingKey(t, env.URL, rpcType)

	serveCtx, serveCancel := context.WithCancel(ctx)
	defer serveCancel()

	var startedOnce sync.Once
	started := make(chan struct{})
	handlers := map[string]rmq.RPCHandlerFunc{
		rpcType: func(_ context.Context, e rmq.Envelope) (any, error) {
			startedOnce.Do(func() { close(started) })
			var req rpcRequest
			if err := e.Decode(&req); err != nil {
				return nil, err
			}
			return rpcResponse{A: "echo:" + req.Q}, nil
		},
	}
	go func() {
		_ = env.Client.ServeRPC(serveCtx, "worker.rpc", handlers)
	}()

	callCtx, callCancel := context.WithTimeout(ctx, 10*time.Second)
	defer callCancel()

	var resp rpcResponse
	if err := env.Client.Call(callCtx, "wa.rpc", rpcType, rpcRequest{Q: "ping"}, &resp); err != nil {
		t.Fatalf("rpc call: %v", err)
	}

	if resp.A != "echo:ping" {
		t.Fatalf("unexpected rpc response: %q", resp.A)
	}

	select {
	case <-started:
	default:
		t.Fatal("rpc handler was never invoked")
	}
}

func bindTestQueue(t *testing.T, url string) {
	t.Helper()
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	if _, err := ch.QueueDeclare(testQueue, true, false, false, false, nil); err != nil {
		t.Fatalf("queue declare: %v", err)
	}
	if err := ch.QueueBind(testQueue, "wa.event.#", testExchange, false, nil); err != nil {
		t.Fatalf("queue bind: %v", err)
	}
}

func bindRPCRoutingKey(t *testing.T, url, routingKey string) {
	t.Helper()
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.QueueBind("worker.rpc", routingKey, "wa.rpc", false, nil); err != nil {
		t.Fatalf("rpc queue bind: %v", err)
	}
}
