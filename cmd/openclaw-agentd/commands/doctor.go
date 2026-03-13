package commands

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/config"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic checks and report configuration health",
		Long: `doctor checks all prerequisites and configuration health:

  - cloudflared binary
  - Keychain / secret storage access
  - Config file existence and validity
  - Agent ID and public key
  - Network connectivity to agentboard
  - Platform compatibility`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	}
}

type checkResult struct {
	name   string
	ok     bool
	detail string
}

func runDoctor() error {
	fmt.Println("🩺  openclaw-agentd doctor")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	checks := []checkResult{}

	// 1. Platform.
	checks = append(checks, checkItem("Platform",
		runtime.GOOS == "darwin" || runtime.GOOS == "linux",
		fmt.Sprintf("%s/%s (darwin recommended for Keychain)", runtime.GOOS, runtime.GOARCH)))

	// 2. cloudflared installed.
	_, cfErr := exec.LookPath("cloudflared")
	checks = append(checks, checkItem("cloudflared binary",
		cfErr == nil,
		binaryDetail("cloudflared", cfErr)))

	// 3. Config file exists.
	cfgPath, _ := config.ConfigPath()
	_, statErr := os.Stat(cfgPath)
	checks = append(checks, checkItem("Config file", statErr == nil, cfgPath))

	// 4. Agent initialized.
	checks = append(checks, checkItem("Agent initialized",
		cfg.AgentID != "",
		agentInitDetail()))

	// 5. Public key in config.
	checks = append(checks, checkItem("Public key configured",
		cfg.PublicKey != "",
		publicKeyDetail()))

	// 6. Private key in Keychain.
	_, keyErr := keychain.Load(keychain.PrivKeyAccount)
	checks = append(checks, checkItem("Private key in secure storage",
		keyErr == nil,
		keystoreDetail(keyErr)))

	// 7. Public hostname configured.
	checks = append(checks, checkItem("Public hostname configured",
		cfg.PublicHostname != "",
		hostnameDetail()))

	// 8. Tunnel name configured.
	checks = append(checks, checkItem("Tunnel name configured",
		cfg.CloudflareTunnelName != "",
		fmt.Sprintf("tunnel_name=%s", cfg.CloudflareTunnelName)))

	// 9. Network connectivity to agentboard.
	alive, _ := probeHealth(cfg.RegistrationAPIURL + "/health")
	checks = append(checks, checkItem("agentboard reachable",
		alive,
		fmt.Sprintf("%s/health", cfg.RegistrationAPIURL)))

	// Print results.
	allOK := true
	for _, c := range checks {
		icon := "✓"
		if !c.ok {
			icon = "✗"
			allOK = false
		}
		fmt.Printf("  %s  %-35s %s\n", icon, c.name, c.detail)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if allOK {
		fmt.Println("✅  All checks passed.")
	} else {
		fmt.Println("⚠   Some checks failed. See details above.")
		fmt.Println()
		fmt.Println("Suggested fixes:")
		for _, c := range checks {
			if !c.ok {
				printFix(c.name)
			}
		}
	}
	return nil
}

func checkItem(name string, ok bool, detail string) checkResult {
	return checkResult{name: name, ok: ok, detail: detail}
}

func binaryDetail(name string, err error) string {
	if err == nil {
		p, _ := exec.LookPath(name)
		return p
	}
	return "not found — brew install cloudflare/cloudflare/cloudflared"
}

func agentInitDetail() string {
	if cfg.AgentID == "" {
		return "run: openclaw-agentd init"
	}
	return cfg.AgentID
}

func publicKeyDetail() string {
	if cfg.PublicKey == "" {
		return "run: openclaw-agentd init"
	}
	if len(cfg.PublicKey) > 16 {
		return cfg.PublicKey[:8] + "..." + cfg.PublicKey[len(cfg.PublicKey)-8:]
	}
	return cfg.PublicKey
}

func keystoreDetail(err error) string {
	if err == nil {
		return "ok"
	}
	return "missing — run: openclaw-agentd init"
}

func hostnameDetail() string {
	if cfg.PublicHostname == "" {
		return "run: openclaw-agentd configure --hostname ..."
	}
	return cfg.PublicHostname
}

func printFix(checkName string) {
	fixes := map[string]string{
		"cloudflared binary":            "  brew install cloudflare/cloudflare/cloudflared",
		"Agent initialized":             "  openclaw-agentd init",
		"Public key configured":         "  openclaw-agentd init",
		"Private key in secure storage": "  openclaw-agentd init",
		"Public hostname configured":    "  openclaw-agentd configure --hostname agent-<id>.<yourdomain.com>",
		"Tunnel name configured":        "  openclaw-agentd init",
		"agentboard reachable":          "  Check network connectivity to agentboard.burmaster.com",
		"Config file":                   "  openclaw-agentd init",
	}
	if fix, ok := fixes[checkName]; ok {
		fmt.Printf("  • %s\n    %s\n", checkName, fix)
	}
}
