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

		// claude: auto-populate user and server from OS if not explicitly set
		if cfg.User == "" {
			cfg.User = identity.Username()
		}
		if cfg.Server == "" {
			cfg.Server = identity.Hostname()
		}

		// claude: regenerate ID from OS defaults unless --id was explicitly passed;
		// ResolveToken may have set cfg.ID from config file, which is stale for register
		if !cmd.Flags().Changed("id") {
			if cmd.Flags().Changed("name") {
				cfg.ID = cfg.Name
			} else {
				cfg.ID = identity.DefaultWorkerName(cfg.User, cfg.Server)
			}
		}
		if cfg.Origin == "" {
			cfg.Origin = identity.Origin()
		}

		opts := []pubsub.Option{
			pubsub.WithID(cfg.ID),
			pubsub.WithToken(cfg.Token),
			pubsub.WithBaseURL(cfg.URL),
			pubsub.WithOrigin(cfg.Origin),
			pubsub.WithUser(cfg.User),
		}

		if !cfg.NoServer {
			opts = append(opts, pubsub.WithServer(cfg.Server))
			if cfg.ServerMID != "" {
				opts = append(opts, pubsub.WithServerMID(cfg.ServerMID))
			}
		}

		ps := pubsub.NewClient(opts...)

		result, err := ps.Register(context.Background(), cfg.Passkey)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		if result.Error != "" {
			return fmt.Errorf("error registering worker: %s", result.Error)
		}

		fmt.Printf("Registration token: %s\n", result.Token)
		fmt.Printf("\nStart the worker with:\n  cla-worker run --token %s --id %s\n\n", result.Token, cfg.ID)
		fmt.Printf("To remove this registration:\n  cla-worker unregister --token %s --id %s\n\n", result.Token, cfg.ID)

		save, _ := cmd.Flags().GetBool("save")
		if save {
			// claude: override save format if --save-format was specified
			if saveFormat, _ := cmd.Flags().GetString("save-format"); saveFormat != "" {
				cfg.SetSaveFormat(saveFormat)
			}
			err := cfg.Save(map[string]interface{}{
				"registrations": []config.Registration{{ID: cfg.ID, Token: result.Token, URL: cfg.URL}},
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
	registerCmd.Flags().String("save-format", "", "config format for --save (yaml or toml, default: toml)")
	registerCmd.Flags().String("url", "", "base server URL")
	registerCmd.Flags().String("id", "", "worker ID")
	registerCmd.Flags().String("origin", "", "origin identifier")
	registerCmd.Flags().StringSlice("tags", nil, "worker tags")
	registerCmd.Flags().String("server", "", "bind to a server resource by name (default: OS hostname)")
	registerCmd.Flags().String("server-mid", "", "bind to a server resource by MID")
	registerCmd.Flags().Bool("no-server", false, "do not bind to any server")
	registerCmd.Flags().String("user", "", "OS user for the worker (default: current user)")
	registerCmd.Flags().String("name", "", "worker name (default: user@hostname)")

	rootCmd.AddCommand(registerCmd)
}
