package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/audit"
	"github.com/burmaster/openclaw-agentd/internal/config"
	"github.com/burmaster/openclaw-agentd/internal/crypto"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
)

func newInitCmd() *cobra.Command {
	var agentName string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the agent: generate keys, create config",
		Long: `init generates an Ed25519 keypair, stores the private key in the
macOS Keychain (or a mode-600 file on other platforms), and writes the
initial configuration to ~/.config/openclaw-agentd/config.yaml.

Run this once before any other commands.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(agentName, force)
		},
	}

	cmd.Flags().StringVar(&agentName, "name", "", "agent name (defaults to hostname-based)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config and regenerate keys")
	return cmd
}

func runInit(agentName string, force bool) error {
	// Check for existing config.
	existingCfg, _ := config.Load()
	if existingCfg.AgentID != "" && !force {
		return fmt.Errorf("agent already initialized (id=%s). Use --force to reinitialize", existingCfg.AgentID)
	}

	if !force {
		fmt.Println("Initializing openclaw-agentd...")
		fmt.Println()
	} else {
		fmt.Println("⚠  Reinitializing — existing keys and config will be replaced.")
		if !confirm("Are you sure?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Generate agent ID (short random hex).
	shortID, err := generateShortID()
	if err != nil {
		return fmt.Errorf("generating agent ID: %w", err)
	}
	agentID := "agent-" + shortID

	// Determine agent name.
	if agentName == "" {
		agentName = "openclaw-" + shortID
	}
	agentName = sanitizeAgentName(agentName)

	// Generate Ed25519 keypair.
	fmt.Print("→ Generating Ed25519 keypair...")
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("generating keypair: %w", err)
	}
	fmt.Println(" ✓")

	// Store private key in Keychain.
	fmt.Print("→ Storing private key in secure storage...")
	if err := keychain.Store(keychain.PrivKeyAccount, kp.PrivateKeyBytes()); err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("storing private key: %w", err)
	}
	fmt.Println(" ✓")

	// Build and save config.
	newCfg := config.DefaultConfig()
	newCfg.AgentID = agentID
	newCfg.AgentName = agentName
	newCfg.PublicKey = kp.PublicKeyHex()
	newCfg.CloudflareTunnelName = "openclaw-agent-" + shortID

	fmt.Print("→ Writing config...")
	if err := config.Save(newCfg); err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Println(" ✓")

	// Initialize audit log.
	auditLog, err := audit.New(agentID, "")
	if err != nil {
		return fmt.Errorf("creating audit logger: %w", err)
	}
	auditLog.MustLog(audit.EventInit, "agent initialized", map[string]string{
		"agent_id":   agentID,
		"agent_name": agentName,
	})
	auditLog.MustLog(audit.EventKeyGenerated, "Ed25519 keypair generated", map[string]string{
		"public_key": kp.PublicKeyHex(),
	})

	fmt.Println()
	fmt.Printf("✅  Agent initialized!\n")
	fmt.Printf("   Agent ID   : %s\n", agentID)
	fmt.Printf("   Agent Name : %s\n", agentName)
	fmt.Printf("   Public Key : %s\n", kp.PublicKeyHex())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. openclaw-agentd login        # authenticate with Cloudflare")
	fmt.Println("  2. openclaw-agentd configure    # set public hostname")
	fmt.Println("  3. openclaw-agentd expose       # start tunnel + register")
	return nil
}

// generateShortID creates an 8-character hex random ID.
func generateShortID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// sanitizeAgentName replaces characters not suitable for DNS hostnames.
func sanitizeAgentName(name string) string {
	r := strings.NewReplacer(" ", "-", "_", "-", ".", "-")
	name = r.Replace(strings.ToLower(name))
	// Truncate to 40 chars.
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}
