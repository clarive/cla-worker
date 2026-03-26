package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	svc "github.com/clarive/cla-worker-go/internal/service"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the Clarive Worker service",
	RunE: func(cmd *cobra.Command, args []string) error {
		// claude: stop the service before removing to avoid orphaned processes
		_ = svc.Stop()
		if err := svc.Remove(); err != nil {
			return fmt.Errorf("removing service: %w", err)
		}
		fmt.Println("service removed")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
