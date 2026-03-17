package dispatcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/clarive/cla-worker-go/internal/pubsub"
	"github.com/stretchr/testify/assert"
)

func TestDispatcher_RoutesAllCommands(t *testing.T) {
	commands := []string{
		"worker.ready",
		"worker.exec",
		"worker.eval",
		"worker.capable",
		"worker.file_exists",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			mp := &mockPublisher{}
			d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, []string{"linux"}, "w1", nil)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			ch := make(chan pubsub.Message, 1)
			ch <- pubsub.Message{Event: cmd, OID: "oid-1", Data: map[string]interface{}{
				"cmd":  "echo test",
				"code": "1+1",
				"tags": []interface{}{"linux"},
				"path": "/tmp",
			}}
			close(ch)

			d.Run(ctx, ch)

			acks := mp.getEvents()
			found := false
			for _, e := range acks {
				if e == cmd+".ack" {
					found = true
					break
				}
			}
			assert.True(t, found, "expected ack for %s", cmd)
		})
	}
}

func TestDispatcher_UnknownCommand(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := make(chan pubsub.Message, 1)
	ch <- pubsub.Message{Event: "worker.unknown", OID: "oid-1", Data: map[string]interface{}{}}
	close(ch)

	d.Run(ctx, ch)

	pubs := mp.getPayloads()
	found := false
	for _, p := range pubs {
		if rc, ok := p["rc"]; ok && rc == 99 {
			found = true
		}
	}
	assert.True(t, found, "expected error for unknown command")
}

func TestDispatcher_ShutdownNoAck(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil)
	cancelled := false
	d.SetCancelFunc(func() { cancelled = true })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := make(chan pubsub.Message, 1)
	ch <- pubsub.Message{Event: "worker.shutdown", OID: "oid-1", Data: map[string]interface{}{
		"reason": "test",
	}}
	close(ch)

	d.Run(ctx, ch)

	acks := mp.getEvents()
	for _, e := range acks {
		assert.NotEqual(t, "worker.shutdown.ack", e, "shutdown should not send ack")
	}
	assert.True(t, cancelled, "cancel func should be called")
	assert.Equal(t, 10, d.ShutdownCode())
}

func TestDispatcher_ConcurrentMessages(t *testing.T) {
	mp := &mockPublisher{}
	me := &mockExecutor{delay: 10 * time.Millisecond}
	d := New(mp, me, &mockFS{}, &mockEval{}, nil, "w1", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := make(chan pubsub.Message, 10)
	for i := 0; i < 5; i++ {
		ch <- pubsub.Message{
			Event: "worker.exec",
			OID:   fmt.Sprintf("oid-%d", i),
			Data:  map[string]interface{}{"cmd": "echo test"},
		}
	}
	close(ch)

	d.Run(ctx, ch)

	pubs := mp.getPayloads()
	doneCount := 0
	for _, p := range pubs {
		if ev, ok := p["_event"]; ok && ev == "worker.done" {
			doneCount++
		}
	}
	assert.Equal(t, 5, doneCount)
}

func TestDispatcher_ContextCancel(t *testing.T) {
	mp := &mockPublisher{}
	d := New(mp, &mockExecutor{}, &mockFS{}, &mockEval{}, nil, "w1", nil)

	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan pubsub.Message)
	done := make(chan struct{})
	go func() {
		d.Run(ctx, ch)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher should stop on context cancel")
	}
}
