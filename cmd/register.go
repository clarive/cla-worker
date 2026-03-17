package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/config"
	"github.com/clarive/cla-worker-go/internal/identity"
	"github.com/clarive/cla-worker-go/internal/pubsub"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register worker with a passkey",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Passkey == "" {
			return fmt.Errorf("passkey is required (use --passkey)")
		}

		if cfg.ID == "" {
			cfg.ID = identity.WorkerID()
		}
		if cfg.Origin == "" {
			cfg.Origin = identity.Origin()
		}

		ps := pubsub.NewClient(
			pubsub.WithID(cfg.ID),
			pubsub.WithToken(cfg.Token),
			pubsub.WithBaseURL(cfg.URL),
			pubsub.WithOrigin(cfg.Origin),
		)

		result, err := ps.Register(context.Background(), cfg.Passkey)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		if result.Error != "" {
			return fmt.Errorf("error registering worker: %s", result.Error)
		}

		fmt.Printf("Registration token: %s\n", result.Token)
		fmt.Printf("Projects registered: %v\n", result.Projects)
		fmt.Printf("\nStart the worker with:\n  cla-worker run --token %s --id %s\n\n", result.Token, cfg.ID)
		fmt.Printf("To remove this registration:\n  cla-worker unregister --token %s --id %s\n\n", result.Token, cfg.ID)

		save, _ := cmd.Flags().GetBool("save")
		if save {
			err := cfg.Save(map[string]interface{}{
				"registrations": []config.Registration{{ID: cfg.ID, Token: result.Token}},
			})
			if err != nil {
				return fmt.Errorf("saving registration: %w", err)
			}
			fmt.Printf("registration saved to config file '%s'\n", cfg.FilePath())
		}

		return nil
	},
}

func init() {
	registerCmd.Flags().String("passkey", "", "server registration passkey")
	registerCmd.Flags().Bool("save", false, "save registration to config file")
	registerCmd.Flags().String("url", "", "base server URL")
	registerCmd.Flags().String("id", "", "worker ID")
	registerCmd.Flags().String("origin", "", "origin identifier")
	registerCmd.Flags().StringSlice("tags", nil, "worker tags")

	rootCmd.AddCommand(registerCmd)
}
