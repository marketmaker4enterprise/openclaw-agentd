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

func newRegisterCmd() *cobra.Command {
	var (
		hostname string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this agent with agentboard.burmaster.com",
		Long: `register performs the A2A challenge-response registration flow with
agentboard.burmaster.com independently of 'expose'.

Use this if you want to re-register or if 'expose --no-register' was used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(hostname, force)
		},
	}

	cmd.Flags().StringVar(&hostname, "hostname", "", "override public hostname for registration")
	cmd.Flags().BoolVar(&force, "force", false, "re-register even if already registered")
	return cmd
}

func runRegister(hostname string, force bool) error {
	if cfg.AgentID == "" {
		return fmt.Errorf("agent not initialized — run 'openclaw-agentd init' first")
	}

	h := cfg.PublicHostname
	if hostname != "" {
		h = hostname
	}
	if h == "" {
		return fmt.Errorf("no public hostname set — use --hostname or 'openclaw-agentd configure --hostname ...'")
	}

	// Load private key.
	privKeyBytes, err := keychain.Load(keychain.PrivKeyAccount)
	if err != nil {
		return fmt.Errorf("loading private key: %w", err)
	}
	privKey, err := crypto.PrivateKeyFromBytes(privKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Audit.
	auditLog, _ := audit.New(cfg.AgentID, "")

	fmt.Printf("→ Registering %s (%s) with %s...\n", cfg.AgentName, cfg.AgentID, cfg.RegistrationAPIURL)

	rc := registry.NewClient(cfg.RegistrationAPIURL, privKey, pubKey)
	agentID, err := rc.Register(cfg.AgentName, h, cfg.Capabilities, agent.Version)
	if err != nil {
		auditLog.MustLog(audit.EventRegistered, "registration failed: "+err.Error(), nil)
		return fmt.Errorf("registration failed: %w", err)
	}

	auditLog.MustLog(audit.EventRegistered, "registered with agentboard", map[string]string{
		"agent_id": agentID,
		"hostname": h,
	})

	// Update config if the server assigned a different ID.
	if agentID != cfg.AgentID {
		fmt.Printf("   Server assigned agent ID: %s\n", agentID)
		cfg.AgentID = agentID
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving updated agent ID: %w", err)
		}
	}

	fmt.Printf("✅  Registered! Agent ID: %s\n", agentID)
	return nil
}
