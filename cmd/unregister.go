package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/identity"
	"github.com/clarive/cla-worker-go/internal/pubsub"
)

var unregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "Unregister worker from server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Origin == "" {
			cfg.Origin = identity.Origin()
		}

		ps := pubsub.NewClient(
			pubsub.WithID(cfg.ID),
			pubsub.WithToken(cfg.Token),
			pubsub.WithBaseURL(cfg.URL),
			pubsub.WithOrigin(cfg.Origin),
		)

		if err := ps.Unregister(context.Background()); err != nil {
			return fmt.Errorf("unregister failed: %w", err)
		}

		fmt.Printf("worker registration removed: %s\n", cfg.ID)
		return nil
	},
}

func init() {
	unregisterCmd.Flags().String("url", "", "base server URL")
	unregisterCmd.Flags().String("id", "", "worker ID")
	unregisterCmd.Flags().String("token", "", "connection token")
	unregisterCmd.Flags().String("origin", "", "origin identifier")

	rootCmd.AddCommand(unregisterCmd)
}
