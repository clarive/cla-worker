package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/clarive/cla-worker-go/internal/pubsub"
)

type Dispatcher struct {
	publisher    Publisher
	executor     CommandExecutor
	filesystem   FileSystem
	evaluator    JSEvaluator
	tags         []string
	workerID     string
	logger       *slog.Logger
	wg           sync.WaitGroup
	cancelFunc   context.CancelFunc
	shutdownCode int
	seenConnect  bool
}

func New(
	publisher Publisher,
	executor CommandExecutor,
	filesystem FileSystem,
	evaluator JSEvaluator,
	tags []string,
	workerID string,
	logger *slog.Logger,
) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		publisher:  publisher,
		executor:   executor,
		filesystem: filesystem,
		evaluator:  evaluator,
		tags:       tags,
		workerID:   workerID,
		logger:     logger,
	}
}

func (d *Dispatcher) SetCancelFunc(cancel context.CancelFunc) {
	d.cancelFunc = cancel
}

func (d *Dispatcher) ShutdownCode() int {
	return d.shutdownCode
}

func (d *Dispatcher) Run(ctx context.Context, messages <-chan pubsub.Message) {
	for {
		select {
		case <-ctx.Done():
			d.wg.Wait()
			return
		case msg, ok := <-messages:
			if !ok {
				d.wg.Wait()
				return
			}
			d.wg.Add(1)
			go func(m pubsub.Message) {
				defer d.wg.Done()
				d.dispatch(ctx, m)
			}(msg)
		}
	}
}

func (d *Dispatcher) isNotificationEvent(event string) bool {
	switch event {
	case "worker.connect", "worker.disconnect", "worker.register", "worker.unregister":
		return true
	}
	return false
}

func (d *Dispatcher) dispatch(ctx context.Context, msg pubsub.Message) {
	if d.isNotificationEvent(msg.Event) {
		// claude: log first worker.connect at INFO so the user sees the connection,
		// then demote subsequent notification events to DEBUG to avoid log spam
		if msg.Event == "worker.connect" && !d.seenConnect {
			d.seenConnect = true
			d.logger.Info("dispatching message", "event", msg.Event, "oid", msg.OID)
		} else {
			d.logger.Debug("dispatching message", "event", msg.Event, "oid", msg.OID)
		}
	} else {
		d.logger.Info("dispatching message", "event", msg.Event, "oid", msg.OID)
	}

	if msg.Event != "worker.shutdown" {
		d.publisher.Publish(ctx, msg.Event+".ack", map[string]interface{}{
			"oid": msg.OID,
		})
	}

	switch msg.Event {
	case "worker.ready":
		d.handleReady(ctx, msg.OID, msg.Data)
	case "worker.exec":
		d.handleExec(ctx, msg.OID, msg.Data)
	case "worker.eval":
		d.handleEval(ctx, msg.OID, msg.Data)
	case "worker.put_file":
		d.handlePutFile(ctx, msg.OID, msg.Data)
	case "worker.get_file":
		d.handleGetFile(ctx, msg.OID, msg.Data)
	case "worker.capable":
		d.handleCapable(ctx, msg.OID, msg.Data)
	case "worker.file_exists":
		d.handleFileExists(ctx, msg.OID, msg.Data)
	case "worker.shutdown":
		d.handleShutdown(ctx, msg.OID, msg.Data)
	case "worker.connect", "worker.disconnect", "worker.register", "worker.unregister":
		// claude: server-side notification events — already logged above
	default:
		d.publishError(ctx, msg.OID, msg.Event,
			fmt.Sprintf("invalid command %s in message id=%s", msg.Event, msg.OID))
	}
}

func (d *Dispatcher) Wait() {
	d.wg.Wait()
}
