package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/daemon"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Reports the worker daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.ID == "" {
			return fmt.Errorf("worker id is not defined; use --id or configure cla-worker.yml")
		}

		fmt.Printf("checking status for workerId=%s pidfile=%s\n", cfg.ID, cfg.Pidfile)

		status := daemon.GetStatus(cfg.Pidfile)

		if status.Running {
			fmt.Printf("worker is running with pid=%d\n", status.PID)
		} else {
			fmt.Println("worker is not running")
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().String("id", "", "worker ID")
	statusCmd.Flags().String("pidfile", "", "path to PID file")
	statusCmd.Flags().String("logfile", "", "path to log file")

	rootCmd.AddCommand(statusCmd)
}
