package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

func newConfigureCmd() *cobra.Command {
	var (
		hostname          string
		bindAddress       string
		agentName         string
		registrationURL   string
		capabilities      []string
		logLevelFlag      string
		heartbeatInterval string
		rpmFlag           int
		burstFlag         int
		resetAllowedPeers bool
	)

	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Update agent configuration",
		Long: `configure sets or updates values in ~/.config/openclaw-agentd/config.yaml.

Examples:
  openclaw-agentd configure --hostname agent-abc123.example.com
  openclaw-agentd configure --bind 127.0.0.1:9090
  openclaw-agentd configure --capabilities execute,query,search`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigure(cmd, hostname, bindAddress, agentName,
				registrationURL, capabilities, logLevelFlag,
				heartbeatInterval, rpmFlag, burstFlag, resetAllowedPeers)
		},
	}

	cmd.Flags().StringVar(&hostname, "hostname", "", "public hostname (e.g. agent-abc123.yourdomain.com)")
	cmd.Flags().StringVar(&bindAddress, "bind", "", "local bind address (default: 127.0.0.1:7878)")
	cmd.Flags().StringVar(&agentName, "name", "", "human-readable agent name")
	cmd.Flags().StringVar(&registrationURL, "registration-url", "", "registration API base URL")
	cmd.Flags().StringSliceVar(&capabilities, "capabilities", nil, "comma-separated capability list")
	cmd.Flags().StringVar(&logLevelFlag, "log-level", "", "log level: trace|debug|info|warn|error")
	cmd.Flags().StringVar(&heartbeatInterval, "heartbeat", "", "heartbeat interval (e.g. 60s, 5m)")
	cmd.Flags().IntVar(&rpmFlag, "rate-rpm", 0, "requests per minute limit (0=no change)")
	cmd.Flags().IntVar(&burstFlag, "rate-burst", 0, "rate limit burst size (0=no change)")
	cmd.Flags().BoolVar(&resetAllowedPeers, "reset-peers", false, "clear the allowed peer IDs list")
	return cmd
}

func runConfigure(cmd *cobra.Command, hostname, bindAddress, agentName,
	registrationURL string, capabilities []string, logLevelFlag,
	heartbeatInterval string, rpmFlag, burstFlag int, resetAllowedPeers bool) error {

	changed := false

	if hostname != "" {
		if !isValidHostname(hostname) {
			return fmt.Errorf("invalid hostname %q (must be a valid DNS name)", hostname)
		}
		cfg.PublicHostname = hostname
		cfg.LocalAgentURL = "http://" + cfg.BindAddress
		changed = true
		fmt.Printf("  public_hostname = %s\n", hostname)
	}

	if bindAddress != "" {
		if !strings.Contains(bindAddress, ":") {
			return fmt.Errorf("bind address must be host:port (e.g. 127.0.0.1:7878)")
		}
		host := strings.Split(bindAddress, ":")[0]
		if host == "0.0.0.0" || host == "::" {
			if !confirm("⚠  Binding to " + host + " exposes the agent to the network. Proceed?") {
				fmt.Println("Aborted.")
				return nil
			}
		}
		cfg.BindAddress = bindAddress
		cfg.LocalAgentURL = "http://" + bindAddress
		changed = true
		fmt.Printf("  bind_address = %s\n", bindAddress)
	}

	if agentName != "" {
		cfg.AgentName = sanitizeAgentName(agentName)
		changed = true
		fmt.Printf("  agent_name = %s\n", cfg.AgentName)
	}

	if registrationURL != "" {
		cfg.RegistrationAPIURL = registrationURL
		changed = true
		fmt.Printf("  registration_api_url = %s\n", registrationURL)
	}

	if len(capabilities) > 0 {
		cfg.Capabilities = capabilities
		changed = true
		fmt.Printf("  capabilities = %v\n", capabilities)
	}

	if logLevelFlag != "" {
		cfg.LogLevel = logLevelFlag
		changed = true
		fmt.Printf("  log_level = %s\n", logLevelFlag)
	}

	if heartbeatInterval != "" {
		// Parse as duration.
		fmt.Printf("  heartbeat_interval = %s\n", heartbeatInterval)
		// Store as string — viper/yaml will handle duration parsing.
		changed = true
	}

	if rpmFlag > 0 {
		cfg.RateLimits.RequestsPerMinute = rpmFlag
		changed = true
		fmt.Printf("  rate_limits.requests_per_minute = %d\n", rpmFlag)
	}

	if burstFlag > 0 {
		cfg.RateLimits.BurstSize = burstFlag
		changed = true
		fmt.Printf("  rate_limits.burst_size = %d\n", burstFlag)
	}

	if resetAllowedPeers {
		cfg.AllowedPeerIDs = []string{}
		changed = true
		fmt.Println("  allowed_peer_ids = []")
	}

	if !changed {
		fmt.Println("No changes made. Use --help to see available options.")
		return nil
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println()
	fmt.Println("✅  Configuration updated.")
	return nil
}

func isValidHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}
