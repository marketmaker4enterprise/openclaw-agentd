package commands

import (
	"crypto/ed25519"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/audit"
	"github.com/burmaster/openclaw-agentd/internal/crypto"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
	"github.com/burmaster/openclaw-agentd/internal/registry"
)

func newRevokeCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Deregister this agent from agentboard and revoke its credentials",
		Long: `revoke notifies agentboard.burmaster.com to deregister this agent
and mark its public key as revoked.

The local config and keys are preserved. Use 'openclaw-agentd uninstall' to
remove everything.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevoke(force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "revoke without confirmation prompt")
	return cmd
}

func runRevoke(force bool) error {
	if cfg.AgentID == "" {
		return fmt.Errorf("agent not initialized")
	}

	if !force {
		fmt.Printf("⚠  This will deregister agent %s (%s) from agentboard.\n", cfg.AgentName, cfg.AgentID)
		if !confirm("Proceed with revocation?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	auditLog, _ := audit.New(cfg.AgentID, "")

	// Load private key for signed revocation request.
	privKeyBytes, err := keychain.Load(keychain.PrivKeyAccount)
	if err != nil {
		return fmt.Errorf("loading private key: %w", err)
	}
	privKey, err := crypto.PrivateKeyFromBytes(privKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}
	pubKey := privKey.Public().(ed25519.PublicKey)

	fmt.Printf("→ Revoking agent %s...", cfg.AgentID)
	rc := registry.NewClient(cfg.RegistrationAPIURL, privKey, pubKey)
	if err := rc.Revoke(cfg.AgentID); err != nil {
		fmt.Println(" ✗")
		auditLog.MustLog(audit.EventRevoked, "revocation failed: "+err.Error(), nil)
		return fmt.Errorf("revocation failed: %w", err)
	}
	fmt.Println(" ✓")

	auditLog.MustLog(audit.EventRevoked, "agent revoked", map[string]string{
		"agent_id": cfg.AgentID,
	})

	fmt.Println()
	fmt.Println("✅  Agent revoked from agentboard.")
	fmt.Println("   Local config and keys are unchanged.")
	fmt.Println("   To clean up everything: openclaw-agentd uninstall")
	return nil
}
