package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/pubsub"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push file to server",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		key, _ := cmd.Flags().GetString("key")

		if file == "" {
			return fmt.Errorf("--file is required")
		}
		if key == "" {
			return fmt.Errorf("--key is required")
		}

		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer f.Close()

		ps := pubsub.NewClient(
			pubsub.WithBaseURL(cfg.URL),
			pubsub.WithID(cfg.ID),
			pubsub.WithToken(cfg.Token),
		)

		if err := ps.Push(context.Background(), key, file, f); err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		fmt.Printf("pushed %s\n", file)
		return nil
	},
}

func init() {
	pushCmd.Flags().String("file", "", "input file")
	pushCmd.Flags().String("key", "", "file key")
	pushCmd.Flags().String("url", "", "base server URL")

	rootCmd.AddCommand(pushCmd)
}
