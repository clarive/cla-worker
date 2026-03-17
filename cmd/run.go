package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/daemon"
	"github.com/clarive/cla-worker-go/internal/worker"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the worker online",
	Long:  "Start the Clarive Worker and connect to the server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		daemonMode, _ := cmd.Flags().GetBool("daemon")

		if daemonMode {
			return runAsDaemon()
		}

		isFork := os.Getenv("CLA_WORKER_FORKED") == "1"
		if isFork {
			return runForked()
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

func runForeground() error {
	logger := slog.Default()
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
