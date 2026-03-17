package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	svc "github.com/clarive/cla-worker-go/internal/service"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Clarive Worker service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := svc.Install(cfg.ID, cfgFile); err != nil {
			return fmt.Errorf("installing service: %w", err)
		}
		fmt.Println("service installed")
		return nil
	},
}

func init() {
	installCmd.Flags().String("id", "", "worker ID")
	rootCmd.AddCommand(installCmd)
}
