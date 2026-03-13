package commands

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/agent"
	"github.com/burmaster/openclaw-agentd/internal/audit"
	"github.com/burmaster/openclaw-agentd/internal/cloudflare"
	"github.com/burmaster/openclaw-agentd/internal/config"
	"github.com/burmaster/openclaw-agentd/internal/crypto"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
	"github.com/burmaster/openclaw-agentd/internal/registry"
)

func newExposeCmd() *cobra.Command {
	var (
		cfBin      string
		skipVerify bool
		noRegister bool
		domain     string
	)

	cmd := &cobra.Command{
		Use:   "expose",
		Short: "Start local agent, create Cloudflare Tunnel, and register with agentboard",
		Long: `expose performs the full startup sequence:

  1. Validates prerequisites (cloudflared, config, keys)
  2. Prompts for confirmation before public exposure
  3. Starts the local agent HTTP server (127.0.0.1 only)
  4. Creates and starts a named Cloudflare Tunnel
  5. Validates the tunnel is reachable
  6. Registers with agentboard.burmaster.com via A2A challenge-response
  7. Starts heartbeat loop
  8. Runs until SIGINT/SIGTERM

Press Ctrl+C to gracefully shut down.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExpose(cfBin, domain, skipVerify, noRegister)
		},
	}

	cmd.Flags().StringVar(&cfBin, "cloudflared", "", "path to cloudflared binary")
	cmd.Flags().StringVar(&domain, "domain", "", "Cloudflare zone domain (e.g. example.com)")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "skip tunnel reachability verification")
	cmd.Flags().BoolVar(&noRegister, "no-register", false, "skip agentboard registration")
	return cmd
}

func runExpose(cfBin, domain string, skipVerify, noRegister bool) error {
	// Validate prerequisites.
	if cfg.AgentID == "" {
		return fmt.Errorf("agent not initialized — run 'openclaw-agentd init' first")
	}

	// Confirm exposure with the user.
	fmt.Printf("⚡ You are about to expose your agent publicly.\n")
	fmt.Printf("   Agent ID   : %s\n", cfg.AgentID)
	fmt.Printf("   Agent Name : %s\n", cfg.AgentName)
	fmt.Printf("   Local bind : %s\n", cfg.BindAddress)
	if cfg.PublicHostname != "" {
		fmt.Printf("   Public URL : https://%s\n", cfg.PublicHostname)
	}
	fmt.Println()
	if !confirm("Expose this agent to the internet?") {
		fmt.Println("Aborted.")
		return nil
	}

	// Audit logger.
	auditLog, err := audit.New(cfg.AgentID, "")
	if err != nil {
		return fmt.Errorf("creating audit logger: %w", err)
	}

	// Load private key.
	privKeyBytes, err := keychain.Load(keychain.PrivKeyAccount)
	if err != nil {
		return fmt.Errorf("loading private key from keychain: %w\n  Run 'openclaw-agentd init' to regenerate keys", err)
	}
	privKey, err := crypto.PrivateKeyFromBytes(privKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Resolve hostname.
	tunnelName := cfg.CloudflareTunnelName
	if tunnelName == "" {
		return fmt.Errorf("cloudflare_tunnel_name not set — run 'openclaw-agentd init'")
	}

	hostname := cfg.PublicHostname
	if hostname == "" {
		if domain == "" {
			return fmt.Errorf("public_hostname not set — run 'openclaw-agentd configure --hostname agent-<id>.<yourdomain.com>' or use --domain flag")
		}
		hostname = tunnelName + "." + domain
		cfg.PublicHostname = hostname
		if saveErr := config.Save(cfg); saveErr != nil {
			logger.Warn().Err(saveErr).Msg("could not persist derived hostname")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Start local agent server.
	fmt.Print("→ Starting local agent server...")
	agentServer, err := agent.New(cfg, logger, auditLog)
	if err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("creating agent server: %w", err)
	}
	serverErrCh := make(chan error, 1)
	go func() { serverErrCh <- agentServer.Start(ctx) }()

	time.Sleep(500 * time.Millisecond)
	select {
	case err := <-serverErrCh:
		fmt.Println(" ✗")
		return fmt.Errorf("agent server failed: %w", err)
	default:
		fmt.Println(" ✓")
	}

	// 2. Validate cloudflared.
	fmt.Print("→ Checking cloudflared...")
	if err := cloudflare.ValidateBinary(cfBin); err != nil {
		fmt.Println(" ✗")
		return err
	}
	fmt.Println(" ✓")

	// 3. Create named tunnel (idempotent — cloudflared returns error if exists, we ignore).
	fmt.Printf("→ Provisioning tunnel %q...", tunnelName)
	if _, err := cloudflare.CreateTunnel(cfBin, tunnelName); err != nil {
		logger.Debug().Err(err).Msg("tunnel create (may already exist, continuing)")
	}
	fmt.Println(" ✓")

	// 4. Start cloudflared tunnel.
	fmt.Printf("→ Starting Cloudflare Tunnel → https://%s ...\n", hostname)
	tunnelMgr := cloudflare.New(cloudflare.TunnelConfig{
		TunnelName:     tunnelName,
		LocalURL:       cfg.LocalAgentURL,
		Hostname:       hostname,
		CloudflaredBin: cfBin,
	}, logger, auditLog)

	if err := tunnelMgr.Start(ctx); err != nil {
		return fmt.Errorf("starting tunnel: %w", err)
	}
	fmt.Println("   Tunnel started ✓")

	// 5. Verify reachability.
	if !skipVerify {
		fmt.Printf("→ Verifying tunnel reachability (up to 30s)...")
		if err := cloudflare.ValidateTunnelReachable(hostname, 30*time.Second); err != nil {
			fmt.Println(" ✗ (proceeding anyway)")
			logger.Warn().Err(err).Msg("tunnel reachability check failed")
		} else {
			fmt.Println(" ✓")
		}
	}

	// 6. Register with agentboard.
	var registeredID string
	if !noRegister {
		fmt.Print("→ Registering with agentboard.burmaster.com...")
		rc := registry.NewClient(cfg.RegistrationAPIURL, privKey, pubKey)
		agentID, regErr := rc.Register(cfg.AgentName, hostname, cfg.Capabilities, agent.Version)
		if regErr != nil {
			fmt.Println(" ✗")
			logger.Warn().Err(regErr).Msg("registration failed — running unregistered")
		} else {
			fmt.Println(" ✓")
			registeredID = agentID
			if agentID != cfg.AgentID {
				cfg.AgentID = agentID
				config.Save(cfg)
			}
		}
	}

	// 7. Heartbeat loop.
	if !noRegister && registeredID != "" {
		go runHeartbeatLoop(ctx, privKey, cfg, registeredID, auditLog)
	}

	// 8. Write PID file.
	writePIDFile()

	fmt.Println()
	fmt.Printf("✅  Agent is live!\n")
	fmt.Printf("   Public URL  : https://%s\n", hostname)
	fmt.Printf("   Agent card  : https://%s/.well-known/agent.json\n", hostname)
	fmt.Printf("   Health      : https://%s/health\n", hostname)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop.")

	// Wait for shutdown.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\n→ Shutting down gracefully...")
	cancel()
	tunnelMgr.Stop()
	agentServer.Stop()
	removePIDFile()
	fmt.Println("✓ Stopped cleanly.")
	return nil
}

func runHeartbeatLoop(ctx context.Context, privKey ed25519.PrivateKey, cfg *config.Config, agentID string, auditLog *audit.Logger) {
	pubKey := privKey.Public().(ed25519.PublicKey)
	rc := registry.NewClient(cfg.RegistrationAPIURL, privKey, pubKey)

	interval := cfg.HeartbeatInterval
	if interval == 0 {
		interval = config.DefaultHeartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := rc.Heartbeat(agentID); err != nil {
				logger.Warn().Err(err).Msg("heartbeat failed")
			} else {
				auditLog.MustLog(audit.EventHeartbeat, "heartbeat sent", nil)
				logger.Debug().Msg("heartbeat ok")
			}
		}
	}
}

func writePIDFile() {
	path, err := config.PIDFilePath()
	if err != nil {
		return
	}
	os.WriteFile(path, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)
}

func removePIDFile() {
	path, _ := config.PIDFilePath()
	if path != "" {
		os.Remove(path)
	}
}
