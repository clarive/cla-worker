package worker

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clarive/cla-worker-go/internal/config"
	"github.com/clarive/cla-worker-go/internal/dispatcher"
	"github.com/clarive/cla-worker-go/internal/executor"
	"github.com/clarive/cla-worker-go/internal/filetransfer"
	"github.com/clarive/cla-worker-go/internal/identity"
	"github.com/clarive/cla-worker-go/internal/jseval"
	"github.com/clarive/cla-worker-go/internal/pubsub"
	"github.com/clarive/cla-worker-go/internal/version"
)

type Worker struct {
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{cfg: cfg, logger: logger}
}

// jsEvalAdapter wraps jseval.Evaluator to satisfy dispatcher.JSEvaluator.
type jsEvalAdapter struct {
	eval *jseval.Evaluator
}

func (a *jsEvalAdapter) Eval(ctx context.Context, code string, stash map[string]interface{}) (*dispatcher.EvalResult, error) {
	result, err := a.eval.Eval(ctx, code, stash)
	if err != nil {
		return nil, err
	}
	return &dispatcher.EvalResult{
		Output: result.Output,
		Error:  result.Error,
		Return: result.Return,
	}, nil
}

func (w *Worker) Run(ctx context.Context) (int, error) {
	cfg := w.cfg

	if cfg.ID == "" {
		cfg.ID = identity.WorkerID()
	}
	if cfg.Origin == "" {
		cfg.Origin = identity.Origin()
	}
	if cfg.Token == "" {
		w.logger.Warn("no token detected", "workerId", cfg.ID)
		w.logger.Warn("register first with: cla-worker register")
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ps := pubsub.NewClient(
		pubsub.WithID(cfg.ID),
		pubsub.WithToken(cfg.Token),
		pubsub.WithBaseURL(cfg.URL),
		pubsub.WithOrigin(cfg.Origin),
		pubsub.WithTags(cfg.Tags),
		pubsub.WithVersion(version.Version),
		pubsub.WithPubSubLogger(w.logger),
		pubsub.WithPubSubReconnectDelay(1*time.Second),
	)

	messages, err := ps.Connect(ctx)
	if err != nil {
		return 1, err
	}

	defer func() {
		w.logger.Info("closing connection to Clarive server...")
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		ps.Close(closeCtx)
		w.logger.Info("worker shutdown completed")
	}()

	w.logger.Info("connected to Clarive server",
		"url", cfg.URL,
		"workerId", cfg.ID,
	)

	exec := executor.NewOsExecutor()
	fs := filetransfer.NewOsFileSystem()
	eval := &jsEvalAdapter{eval: jseval.NewEvaluator(30*time.Second, w.logger)}

	disp := dispatcher.New(ps, exec, fs, eval, cfg.Tags, cfg.ID, w.logger)
	disp.SetCancelFunc(cancel)

	disp.Run(ctx, messages)
	disp.Wait()

	if code := disp.ShutdownCode(); code != 0 {
		return code, nil
	}

	return 0, nil
}

func (w *Worker) RunDaemon(ctx context.Context) (int, error) {
	cfg := w.cfg

	if cfg.Logfile != "" {
		logFile, err := os.OpenFile(cfg.Logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return 1, err
		}
		defer logFile.Close()
		w.logger = slog.New(slog.NewJSONHandler(logFile, nil))
	}

	return w.Run(ctx)
}
