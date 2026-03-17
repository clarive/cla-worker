package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/pubsub"
)

var popCmd = &cobra.Command{
	Use:   "pop",
	Short: "Pop file from server",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		key, _ := cmd.Flags().GetString("key")

		if key == "" {
			return fmt.Errorf("--key is required")
		}

		ps := pubsub.NewClient(
			pubsub.WithBaseURL(cfg.URL),
			pubsub.WithID(cfg.ID),
			pubsub.WithToken(cfg.Token),
		)

		var w *os.File
		if file != "" {
			var err error
			w, err = os.Create(file)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer w.Close()
		} else {
			w = os.Stdout
		}

		if err := ps.Pop(context.Background(), key, w); err != nil {
			return fmt.Errorf("pop failed: %w", err)
		}

		if file != "" {
			fmt.Printf("wrote %s\n", file)
		}
		return nil
	},
}

func init() {
	popCmd.Flags().String("file", "", "output file")
	popCmd.Flags().String("key", "", "file key")
	popCmd.Flags().String("url", "", "base server URL")

	rootCmd.AddCommand(popCmd)
}
