package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/config"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
)

func newUninstallCmd() *cobra.Command {
	var (
		keepConfig bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove all openclaw-agentd data: keys, config, PID file, audit log",
		Long: `uninstall removes:

  - Ed25519 private key from macOS Keychain
  - Cloudflare tunnel token from Keychain
  - Config directory (~/.config/openclaw-agentd/)
  - PID file and audit log

Use --keep-config to preserve the config directory.

NOTE: This does not uninstall the binary. Use 'brew uninstall openclaw-agentd'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(keepConfig, force)
		},
	}
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "preserve config directory")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompt")
	return cmd
}

func runUninstall(keepConfig, force bool) error {
	if !force {
		fmt.Println("⚠  This will permanently remove:")
		fmt.Println("   - Private key from Keychain")
		fmt.Println("   - Cloudflare tunnel token from Keychain")
		if !keepConfig {
			dir, _ := config.ConfigDir()
			fmt.Printf("   - Config directory: %s\n", dir)
		}
		fmt.Println()
		if !confirm("Proceed with uninstall?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	errors := []error{}

	// 1. Remove private key from Keychain.
	fmt.Print("→ Removing private key from Keychain...")
	if err := keychain.Delete(keychain.PrivKeyAccount); err != nil {
		fmt.Println(" ✗ (may already be gone)")
		errors = append(errors, err)
	} else {
		fmt.Println(" ✓")
	}

	// 2. Remove CF token from Keychain.
	cfTokenAccount := keychain.CFTokenAccount
	if cfg.CloudflareTunnelName != "" {
		cfTokenAccount = keychain.CFTokenAccount + "-" + cfg.CloudflareTunnelName
	}
	fmt.Print("→ Removing Cloudflare token from Keychain...")
	if err := keychain.Delete(cfTokenAccount); err != nil {
		fmt.Println(" ✗ (may not exist)")
	} else {
		fmt.Println(" ✓")
	}

	// 3. Remove config directory.
	if !keepConfig {
		dir, err := config.ConfigDir()
		if err == nil {
			fmt.Printf("→ Removing config directory %s...", dir)
			if err := os.RemoveAll(dir); err != nil {
				fmt.Println(" ✗")
				errors = append(errors, err)
			} else {
				fmt.Println(" ✓")
			}
		}
	}

	fmt.Println()
	if len(errors) > 0 {
		fmt.Printf("⚠  Uninstall completed with %d warning(s).\n", len(errors))
	} else {
		fmt.Println("✅  Uninstall complete.")
	}
	fmt.Println("   To remove the binary: brew uninstall openclaw-agentd")
	return nil
}
