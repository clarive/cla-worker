package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/daemon"
	"github.com/clarive/cla-worker-go/internal/identity"
	svc "github.com/clarive/cla-worker-go/internal/service"
	"github.com/clarive/cla-worker-go/internal/worker"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the worker online",
	Long:  "Start the Clarive Worker and connect to the server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// claude: auto-detect worker ID from OS if not explicitly set
		if cfg.ID == "" {
			cfg.ID = identity.DefaultWorkerName(identity.Username(), identity.Hostname())
			fmt.Printf("ℹ using auto-detected worker ID: %s\n", cfg.ID)
		}

		daemonMode, _ := cmd.Flags().GetBool("daemon")

		if daemonMode {
			return runAsDaemon()
		}

		isFork := os.Getenv("CLA_WORKER_FORKED") == "1"
		if isFork {
			return runForked()
		}

		// claude: when started by Windows SCM (or systemd/launchd), go
		// through the service framework so it properly signals "running"
		if !svc.Interactive() {
			return runAsService()
		}

		return runForeground()
	},
}

func init() {
	runCmd.Flags().Bool("daemon", false, "run worker in the background")
	runCmd.Flags().String("url", "", "base server URL")
	runCmd.Flags().String("id", "", "worker ID")
	runCmd.Flags().String("token", "", "connection token")
	runCmd.Flags().StringSlice("tags", nil, "worker tags")
	runCmd.Flags().String("logfile", "", "path to log file")
	runCmd.Flags().String("pidfile", "", "path to PID file")

	rootCmd.AddCommand(runCmd)
}

func logLevel() slog.Level {
	if cfg.Verbose >= 1 {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

func runAsService() error {
	var logger *slog.Logger
	if cfg.Logfile != "" {
		logFile, err := os.OpenFile(cfg.Logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer logFile.Close()
		logger = slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: logLevel(),
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel(),
		}))
	}

	code, err := svc.RunAsService(func(ctx context.Context) (int, error) {
		w := worker.New(cfg, logger)
		return w.Run(ctx)
	}, logger)
	if err != nil {
		return fmt.Errorf("service error: %w", err)
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func runForeground() error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel(),
	}))
	w := worker.New(cfg, logger)

	code, err := w.Run(context.Background())
	if err != nil {
		return fmt.Errorf("worker error: %w", err)
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func runForked() error {
	w := worker.New(cfg, nil)
	if cfg.Logfile != "" {
		daemon.WritePID(cfg.Pidfile, os.Getpid())
	}

	code, err := w.RunDaemon(context.Background())
	if err != nil {
		return err
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func runAsDaemon() error {
	status := daemon.GetStatus(cfg.Pidfile)
	if status.Running {
		return fmt.Errorf("another daemon already running with pid=%d", status.PID)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable: %w", err)
	}

	args := os.Args[1:]
	procAttr := &os.ProcAttr{
		Dir: "",
		Env: append(os.Environ(), "CLA_WORKER_FORKED=1"),
		Files: []*os.File{
			os.Stdin,
			nil,
			nil,
		},
	}

	proc, err := os.StartProcess(exe, append([]string{exe}, args...), procAttr)
	if err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	fmt.Printf("forked child with pid %d\n", proc.Pid)
	proc.Release()
	return nil
}
