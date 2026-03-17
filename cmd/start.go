package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	svc "github.com/clarive/cla-worker-go/internal/service"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Clarive Worker service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := svc.Start(); err != nil {
			return fmt.Errorf("starting service: %w", err)
		}
		fmt.Println("service started")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
