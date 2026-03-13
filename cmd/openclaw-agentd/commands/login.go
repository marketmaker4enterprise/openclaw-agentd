package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/cloudflare"
)

func newLoginCmd() *cobra.Command {
	var cfBin string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Cloudflare (opens browser)",
		Long: `login runs 'cloudflared tunnel login' which opens a browser window
to authenticate your Cloudflare account. This stores the credentials certificate
at ~/.cloudflared/cert.pem.

You must run this before 'expose'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cfBin)
		},
	}

	cmd.Flags().StringVar(&cfBin, "cloudflared", "", "path to cloudflared binary (default: from PATH)")
	return cmd
}

func runLogin(cfBin string) error {
	// Validate cloudflared is installed.
	if err := cloudflare.ValidateBinary(cfBin); err != nil {
		return err
	}

	fmt.Println("Opening browser for Cloudflare authentication...")
	fmt.Println("(If no browser opens, visit the URL shown below and authorize.)")
	fmt.Println()

	if err := cloudflare.Login(cfBin); err != nil {
		return fmt.Errorf("cloudflare login failed: %w", err)
	}

	fmt.Println()
	fmt.Println("✅  Cloudflare login successful.")
	fmt.Println("   Run 'openclaw-agentd configure' to set your public hostname.")
	return nil
}
