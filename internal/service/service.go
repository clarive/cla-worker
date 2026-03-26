package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kardianos/service"
)

// RunFunc is the worker entry point called by the service manager.
type RunFunc func(ctx context.Context) (int, error)

type WorkerService struct {
	logger  *slog.Logger
	runFunc RunFunc
	cancel  context.CancelFunc
	done    chan struct{}
	exitCode int
	exitErr  error
}

func (ws *WorkerService) Start(s service.Service) error {
	if ws.runFunc == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ws.cancel = cancel
	ws.done = make(chan struct{})
	go func() {
		defer close(ws.done)
		ws.exitCode, ws.exitErr = ws.runFunc(ctx)
	}()
	return nil
}

func (ws *WorkerService) Stop(s service.Service) error {
	if ws.cancel != nil {
		ws.cancel()
	}
	if ws.done != nil {
		<-ws.done
	}
	return nil
}

// Interactive returns true when running from a terminal, false when
// running as a system service (Windows SCM, systemd, launchd).
func Interactive() bool {
	return service.Interactive()
}

// RunAsService runs the worker through the platform service manager.
// On Windows this connects to the SCM and properly signals service status.
func RunAsService(fn RunFunc, logger *slog.Logger) (int, error) {
	ws := &WorkerService{
		logger:  logger,
		runFunc: fn,
	}

	svcConfig := &service.Config{
		Name:        "cla-worker",
		DisplayName: "Clarive Worker",
		Description: "Clarive Worker agent service",
	}

	s, err := service.New(ws, svcConfig)
	if err != nil {
		return 1, fmt.Errorf("creating service: %w", err)
	}

	if err := s.Run(); err != nil {
		return 1, fmt.Errorf("running service: %w", err)
	}

	return ws.exitCode, ws.exitErr
}

// isMarkedForDeletion checks for Windows ERROR_SERVICE_MARKED_FOR_DELETE.
func isMarkedForDeletion(err error) bool {
	return err != nil && strings.Contains(err.Error(), "marked for deletion")
}

func Install(workerID string, configPath string) error {
	svcConfig := &service.Config{
		Name:        "cla-worker",
		DisplayName: "Clarive Worker",
		Description: "Clarive Worker agent service",
		Arguments:   []string{"run"},
	}

	if workerID != "" {
		svcConfig.Arguments = append(svcConfig.Arguments, "--id", workerID)
	}
	if configPath != "" {
		// claude: resolve to absolute path so systemd/launchd can find it
		if abs, err := filepath.Abs(configPath); err == nil {
			configPath = abs
		}
		svcConfig.Arguments = append(svcConfig.Arguments, "--config", configPath)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}
	svcConfig.Executable = exe

	s, err := service.New(&WorkerService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	err = s.Install()
	if err == nil {
		return nil
	}

	// claude: on Windows a previous remove may have only marked the
	// service for deletion, or the entry is stale; try to clean up
	if !strings.Contains(err.Error(), "already exists") {
		return err
	}

	_ = s.Stop()
	if unErr := s.Uninstall(); unErr != nil && !isMarkedForDeletion(unErr) {
		return fmt.Errorf("service already exists and could not be removed: %w", unErr)
	}

	// claude: wait for SCM to release the deleted entry and retry
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if retryErr := s.Install(); retryErr == nil {
			return nil
		}
	}
	return fmt.Errorf("service already exists; close services.msc or any other tool holding a handle and retry")
}

func Remove() error {
	svcConfig := &service.Config{
		Name: "cla-worker",
	}

	s, err := service.New(&WorkerService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// claude: stop before uninstall using the same service object
	// to avoid separate SCM handles that delay Windows cleanup
	_ = s.Stop()

	if err := s.Uninstall(); err != nil {
		// claude: "marked for deletion" means a previous remove already
		// requested deletion; treat as success and wait for it to clear
		if !isMarkedForDeletion(err) {
			return err
		}
	}

	// claude: on Windows, DeleteService only marks for deletion; the
	// entry persists until all handles are closed (e.g. services.msc).
	// Poll briefly to confirm the service is actually gone.
	if runtime.GOOS == "windows" {
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			_, err := s.Status()
			if err != nil {
				// claude: service no longer queryable — actually removed
				return nil
			}
		}
		return fmt.Errorf("service marked for deletion but still present; close services.msc or any other tool holding a handle and try again")
	}

	return nil
}

func Start() error {
	svcConfig := &service.Config{
		Name: "cla-worker",
	}

	s, err := service.New(&WorkerService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return s.Start()
}

func Stop() error {
	svcConfig := &service.Config{
		Name: "cla-worker",
	}

	s, err := service.New(&WorkerService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return s.Stop()
}
