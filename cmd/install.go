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
		// claude: copy config to system location so the service can find it
		sysConfigPath := ""
		if cfg.FilePath() != "" {
			var err error
			sysConfigPath, err = cfg.InstallSystemConfig()
			if err != nil {
				return fmt.Errorf("setting up system config: %w", err)
			}
			fmt.Printf("config installed to %s\n", sysConfigPath)
		}

		if err := svc.Install(cfg.ID, sysConfigPath); err != nil {
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
