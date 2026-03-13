// Package commands contains all cobra CLI commands.
package commands

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

var (
	cfgFile  string
	logLevel string
	verbose  bool
	logger   zerolog.Logger
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "openclaw-agentd",
	Short: "OpenClaw Agent Daemon — secure A2A agent endpoint",
	Long: `openclaw-agentd stands up a local OpenClaw-compatible agent endpoint,
exposes it securely via Cloudflare Tunnel, and registers it with agentboard.

Quick start:
  openclaw-agentd init
  openclaw-agentd expose`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initGlobals()
	},
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/openclaw-agentd/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: trace|debug|info|warn|error (overrides config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose (debug) logging")

	rootCmd.AddCommand(
		newInitCmd(),
		newLoginCmd(),
		newConfigureCmd(),
		newExposeCmd(),
		newRegisterCmd(),
		newStatusCmd(),
		newRotateKeysCmd(),
		newRevokeCmd(),
		newStopCmd(),
		newDoctorCmd(),
		newUninstallCmd(),
	)
}

func initGlobals() error {
	// Load config.
	var err error
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine log level.
	level := cfg.LogLevel
	if logLevel != "" {
		level = logLevel
	}
	if verbose {
		level = "debug"
	}
	if level == "" {
		level = config.DefaultLogLevel
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(lvl).
		With().
		Timestamp().
		Logger()

	return nil
}

// confirm prompts the user for a yes/no answer and returns true on "y" or "yes".
func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	var answer string
	fmt.Scanln(&answer)
	return answer == "y" || answer == "yes"
}
