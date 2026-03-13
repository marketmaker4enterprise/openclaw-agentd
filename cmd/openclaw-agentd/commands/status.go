package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

func newStatusCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show agent status, config summary, and recent audit events",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func runStatus(jsonOut bool) error {
	status := collectStatus()

	if jsonOut {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	printStatusHuman(status)
	return nil
}

type agentStatus struct {
	AgentID        string            `json:"agent_id"`
	AgentName      string            `json:"agent_name"`
	PublicHostname string            `json:"public_hostname"`
	BindAddress    string            `json:"bind_address"`
	LocalAlive     bool              `json:"local_alive"`
	PIDRunning     bool              `json:"pid_running"`
	PID            int               `json:"pid,omitempty"`
	PublicKeyShort string            `json:"public_key_short"`
	Capabilities   []string          `json:"capabilities"`
	TunnelName     string            `json:"tunnel_name"`
	RegistrationURL string           `json:"registration_url"`
	ConfigPath     string            `json:"config_path"`
	Health         map[string]string `json:"health,omitempty"`
}

func collectStatus() agentStatus {
	s := agentStatus{
		AgentID:         cfg.AgentID,
		AgentName:       cfg.AgentName,
		PublicHostname:  cfg.PublicHostname,
		BindAddress:     cfg.BindAddress,
		Capabilities:    cfg.Capabilities,
		TunnelName:      cfg.CloudflareTunnelName,
		RegistrationURL: cfg.RegistrationAPIURL,
	}

	// Shorten public key for display.
	if len(cfg.PublicKey) > 16 {
		s.PublicKeyShort = cfg.PublicKey[:8] + "..." + cfg.PublicKey[len(cfg.PublicKey)-8:]
	} else {
		s.PublicKeyShort = cfg.PublicKey
	}

	// Config path.
	if p, err := config.ConfigPath(); err == nil {
		s.ConfigPath = p
	}

	// Check PID file.
	if pidPath, err := config.PIDFilePath(); err == nil {
		if data, err := os.ReadFile(pidPath); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 {
				s.PID = pid
				s.PIDRunning = processExists(pid)
			}
		}
	}

	// Probe local agent.
	if cfg.LocalAgentURL != "" {
		s.LocalAlive, s.Health = probeHealth(cfg.LocalAgentURL + "/health")
	}

	return s
}

func printStatusHuman(s agentStatus) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  openclaw-agentd status")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	runIcon := "○ stopped"
	if s.PIDRunning {
		runIcon = fmt.Sprintf("● running (pid %d)", s.PID)
	}
	fmt.Printf("  Process    : %s\n", runIcon)

	healthIcon := "✗ unreachable"
	if s.LocalAlive {
		healthIcon = "✓ healthy"
	}
	fmt.Printf("  Local API  : %s\n", healthIcon)
	fmt.Printf("  Agent ID   : %s\n", orDefault(s.AgentID, "(not initialized)"))
	fmt.Printf("  Agent Name : %s\n", orDefault(s.AgentName, "(not set)"))
	fmt.Printf("  Public URL : %s\n", orDefault(s.PublicHostname, "(not configured)"))
	fmt.Printf("  Tunnel     : %s\n", orDefault(s.TunnelName, "(not configured)"))
	fmt.Printf("  Public Key : %s\n", orDefault(s.PublicKeyShort, "(none)"))
	fmt.Printf("  Config     : %s\n", s.ConfigPath)
	fmt.Printf("  Registry   : %s\n", s.RegistrationURL)

	if len(s.Capabilities) > 0 {
		fmt.Printf("  Caps       : %s\n", strings.Join(s.Capabilities, ", "))
	}

	if len(s.Health) > 0 {
		fmt.Println()
		fmt.Println("  Health response:")
		for k, v := range s.Health {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func probeHealth(url string) (bool, map[string]string) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result map[string]string
	json.Unmarshal(data, &result)
	return true, result
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; send signal 0 to test existence.
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
