package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Clarive Worker daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Pidfile == "" {
			return fmt.Errorf("pidfile is not configured")
		}

		if err := daemon.Kill(cfg.Pidfile); err != nil {
			return fmt.Errorf("stopping daemon: %w", err)
		}

		fmt.Printf("worker %s stopped\n", cfg.ID)
		return nil
	},
}

func init() {
	stopCmd.Flags().String("pidfile", "", "path to PID file")
	rootCmd.AddCommand(stopCmd)
}
