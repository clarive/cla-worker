package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/clarive/cla-worker-go/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "cla-worker",
	Short: "Clarive Worker agent",
	Long:  "Clarive Worker is a pull-based agent that connects to a Clarive server via SSE.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" {
			return nil
		}
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		mergeFlags(cmd)
		cfg.ResolveTags()
		cfg.ResolveToken()
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&cfgFile, "config", "c", "", "path to config file")
	pf.String("url", "", "base server URL")
	pf.String("id", "", "worker ID")
	pf.String("token", "", "connection token")
	pf.String("passkey", "", "registration passkey")
	pf.StringSlice("tags", nil, "worker tags")
	pf.String("origin", "", "origin identifier")
	pf.CountP("verbose", "v", "increase verbosity")
	pf.String("logfile", "", "path to log file")
	pf.String("pidfile", "", "path to PID file")
}

func mergeFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	if flags.Changed("url") {
		v, _ := flags.GetString("url")
		cfg.URL = v
	}
	if flags.Changed("id") {
		v, _ := flags.GetString("id")
		cfg.ID = v
	}
	if flags.Changed("token") {
		v, _ := flags.GetString("token")
		cfg.Token = v
	}
	if flags.Changed("passkey") {
		v, _ := flags.GetString("passkey")
		cfg.Passkey = v
	}
	if flags.Changed("tags") {
		v, _ := flags.GetStringSlice("tags")
		cfg.Tags = v
	}
	if flags.Changed("origin") {
		v, _ := flags.GetString("origin")
		cfg.Origin = v
	}
	if flags.Changed("verbose") {
		v, _ := flags.GetCount("verbose")
		cfg.Verbose = v
	}
	if flags.Changed("logfile") {
		v, _ := flags.GetString("logfile")
		cfg.Logfile = v
	}
	if flags.Changed("pidfile") {
		v, _ := flags.GetString("pidfile")
		cfg.Pidfile = v
	}
	if flags.Changed("server") {
		v, _ := flags.GetString("server")
		cfg.Server = v
	}
	if flags.Changed("server-mid") {
		v, _ := flags.GetString("server-mid")
		cfg.ServerMID = v
	}
	if flags.Changed("no-server") {
		v, _ := flags.GetBool("no-server")
		cfg.NoServer = v
	}
	if flags.Changed("user") {
		v, _ := flags.GetString("user")
		cfg.User = v
	}
	if flags.Changed("name") {
		v, _ := flags.GetString("name")
		cfg.Name = v
	}
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
