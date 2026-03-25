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
	allowedVerbs map[string]bool
	logger       *slog.Logger
	wg           sync.WaitGroup
	cancelFunc   context.CancelFunc
	shutdownCode int
	seenConnect  bool
	// claude: mutex-protected cancel func for the currently running exec command
	execMu     sync.Mutex
	execCancel context.CancelFunc
}

func New(
	publisher Publisher,
	executor CommandExecutor,
	filesystem FileSystem,
	evaluator JSEvaluator,
	tags []string,
	workerID string,
	allowedVerbs map[string]bool,
	logger *slog.Logger,
) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		publisher:    publisher,
		executor:     executor,
		filesystem:   filesystem,
		evaluator:    evaluator,
		tags:         tags,
		workerID:     workerID,
		allowedVerbs: allowedVerbs,
		logger:       logger,
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

// claude: verbFromEvent extracts the short verb name from a worker event.
// e.g. "worker.exec" -> "exec", "worker.get_file" -> "get_file"
func verbFromEvent(event string) string {
	if len(event) > 7 && event[:7] == "worker." {
		return event[7:]
	}
	return ""
}

// claude: isVerbAllowed checks whether the given event's verb is permitted.
// If allowedVerbs is nil (no restrictions configured), everything is allowed.
func (d *Dispatcher) isVerbAllowed(event string) bool {
	if d.allowedVerbs == nil {
		return true
	}
	verb := verbFromEvent(event)
	return d.allowedVerbs[verb]
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

	if msg.Event != "worker.shutdown" && msg.Event != "worker.rename" {
		d.publisher.Publish(ctx, msg.Event+".ack", map[string]interface{}{
			"oid": msg.OID,
		})
	}

	switch msg.Event {
	case "worker.ready":
		d.handleReady(ctx, msg.OID, msg.Data)
	case "worker.exec", "worker.eval", "worker.put_file", "worker.get_file", "worker.file_exists":
		if !d.isVerbAllowed(msg.Event) {
			verb := verbFromEvent(msg.Event)
			d.logger.Warn("verb denied by configuration", "verb", verb)
			d.publishError(ctx, msg.OID, msg.Event,
				fmt.Sprintf("verb %q is not allowed on this worker", verb))
			d.publishDone(ctx, msg.OID)
			return
		}
		switch msg.Event {
		case "worker.exec":
			d.handleExec(ctx, msg.OID, msg.Data)
		case "worker.eval":
			d.handleEval(ctx, msg.OID, msg.Data)
		case "worker.put_file":
			d.handlePutFile(ctx, msg.OID, msg.Data)
		case "worker.get_file":
			d.handleGetFile(ctx, msg.OID, msg.Data)
		case "worker.file_exists":
			d.handleFileExists(ctx, msg.OID, msg.Data)
		}
	case "worker.capable":
		d.handleCapable(ctx, msg.OID, msg.Data)
	case "worker.shutdown":
		d.handleShutdown(ctx, msg.OID, msg.Data)
	case "worker.exec.cancel":
		d.handleExecCancel(ctx, msg.OID, msg.Data)
	case "worker.rename":
		// claude: server renamed this worker — update identity for future reconnects
		oldName, _ := msg.Data["old_name"].(string)
		newName, _ := msg.Data["new_name"].(string)
		d.logger.Warn("renamed worker id", "old_name", oldName, "new_name", newName)
		if newName != "" {
			d.publisher.SetID(newName)
		}
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
