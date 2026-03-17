package service

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/kardianos/service"
)

type WorkerService struct {
	logger *slog.Logger
}

func (ws *WorkerService) Start(s service.Service) error {
	return nil
}

func (ws *WorkerService) Stop(s service.Service) error {
	return nil
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

	return s.Install()
}

func Remove() error {
	svcConfig := &service.Config{
		Name: "cla-worker",
	}

	s, err := service.New(&WorkerService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return s.Uninstall()
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
