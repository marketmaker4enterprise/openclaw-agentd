package commands

import (
	"crypto/ed25519"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/audit"
	"github.com/burmaster/openclaw-agentd/internal/config"
	"github.com/burmaster/openclaw-agentd/internal/crypto"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
	"github.com/burmaster/openclaw-agentd/internal/registry"
	"github.com/burmaster/openclaw-agentd/internal/agent"
)

func newRotateKeysCmd() *cobra.Command {
	var noReregister bool

	cmd := &cobra.Command{
		Use:   "rotate-keys",
		Short: "Generate new Ed25519 keypair and re-register with agentboard",
		Long: `rotate-keys replaces the current Ed25519 private key with a freshly
generated one, updates the Keychain and config, and re-registers with
agentboard so peers get the new public key.

The old key is permanently deleted from the Keychain.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRotateKeys(noReregister)
		},
	}
	cmd.Flags().BoolVar(&noReregister, "no-reregister", false, "rotate key locally without re-registering")
	return cmd
}

func runRotateKeys(noReregister bool) error {
	if cfg.AgentID == "" {
		return fmt.Errorf("agent not initialized — run 'openclaw-agentd init' first")
	}

	auditLog, _ := audit.New(cfg.AgentID, "")

	fmt.Println("⚠  Key rotation replaces your Ed25519 private key.")
	fmt.Println("   Peers will need to update their allowlists with the new public key.")
	fmt.Println()
	if !confirm("Rotate keys now?") {
		fmt.Println("Aborted.")
		return nil
	}

	// 1. Generate new keypair.
	fmt.Print("→ Generating new Ed25519 keypair...")
	newKP, err := crypto.GenerateKeyPair()
	if err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("generating keypair: %w", err)
	}
	fmt.Println(" ✓")

	oldPubKey := cfg.PublicKey

	// 2. Store in Keychain (replaces old key).
	fmt.Print("→ Storing new private key in secure storage...")
	if err := keychain.Store(keychain.PrivKeyAccount, newKP.PrivateKeyBytes()); err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("storing new private key: %w", err)
	}
	fmt.Println(" ✓")

	// 3. Update config with new public key.
	cfg.PublicKey = newKP.PublicKeyHex()
	fmt.Print("→ Saving updated config...")
	if err := config.Save(cfg); err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Println(" ✓")

	auditLog.MustLog(audit.EventKeyRotated, "Ed25519 key rotated", map[string]string{
		"old_public_key": oldPubKey,
		"new_public_key": newKP.PublicKeyHex(),
	})

	// 4. Re-register with agentboard.
	if !noReregister && cfg.PublicHostname != "" {
		fmt.Print("→ Re-registering with agentboard...")
		privKey, err := crypto.PrivateKeyFromBytes(newKP.PrivateKeyBytes())
		if err != nil {
			fmt.Println(" ✗")
			return fmt.Errorf("loading new private key: %w", err)
		}
		pubKey := privKey.Public().(ed25519.PublicKey)
		rc := registry.NewClient(cfg.RegistrationAPIURL, privKey, pubKey)
		agentID, err := rc.Register(cfg.AgentName, cfg.PublicHostname, cfg.Capabilities, agent.Version)
		if err != nil {
			fmt.Println(" ✗")
			fmt.Printf("   Warning: re-registration failed: %v\n", err)
			fmt.Println("   Run 'openclaw-agentd register' manually to retry.")
		} else {
			fmt.Println(" ✓")
			if agentID != cfg.AgentID {
				cfg.AgentID = agentID
				config.Save(cfg)
			}
		}
	}

	fmt.Println()
	fmt.Println("✅  Key rotation complete.")
	fmt.Printf("   New public key: %s\n", newKP.PublicKeyHex())
	fmt.Println()
	fmt.Println("⚠  Update your peer allowlists with the new public key.")
	return nil
}
