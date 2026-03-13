// Package cloudflare manages cloudflared tunnel lifecycle via exec.Command.
package cloudflare

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/burmaster/openclaw-agentd/internal/audit"
	"github.com/burmaster/openclaw-agentd/internal/keychain"
)

// TunnelConfig holds tunnel parameters.
type TunnelConfig struct {
	TunnelName     string // openclaw-agent-<shortid>
	LocalURL       string // http://127.0.0.1:7878
	Hostname       string // agent-<shortid>.<user-domain>
	CloudflaredBin string // path to cloudflared binary
}

// Manager controls a cloudflared tunnel process.
type Manager struct {
	cfg    TunnelConfig
	logger zerolog.Logger
	audit  *audit.Logger
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// New creates a tunnel Manager.
func New(cfg TunnelConfig, logger zerolog.Logger, auditLog *audit.Logger) *Manager {
	if cfg.CloudflaredBin == "" {
		cfg.CloudflaredBin = "cloudflared"
	}
	return &Manager{cfg: cfg, logger: logger, audit: auditLog}
}

// ValidateBinary checks that cloudflared is installed and executable.
func ValidateBinary(bin string) error {
	if bin == "" {
		bin = "cloudflared"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("cloudflared not found in PATH — install via: brew install cloudflare/cloudflare/cloudflared\n  err: %w", err)
	}
	cmd := exec.Command(path, "version")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cloudflared version check failed: %w", err)
	}
	version := strings.TrimSpace(string(out))
	if !strings.HasPrefix(version, "cloudflared version") {
		return fmt.Errorf("unexpected cloudflared version output: %s", version)
	}
	return nil
}

// Login runs `cloudflared tunnel login` to authenticate with Cloudflare.
func Login(bin string) error {
	if bin == "" {
		bin = "cloudflared"
	}
	cmd := exec.Command(bin, "tunnel", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// CreateTunnel runs `cloudflared tunnel create <name>` and returns the tunnel ID.
func CreateTunnel(bin, name string) (string, error) {
	if bin == "" {
		bin = "cloudflared"
	}
	out, err := exec.Command(bin, "tunnel", "create", name).Output()
	if err != nil {
		return "", fmt.Errorf("creating tunnel %q: %w — stdout: %s", name, err, string(out))
	}
	// Parse tunnel ID from output: "Created tunnel <name> with id <uuid>"
	line := string(out)
	for _, part := range strings.Fields(line) {
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			// Looks like a UUID.
			return part, nil
		}
	}
	return "", fmt.Errorf("could not parse tunnel ID from cloudflared output: %s", line)
}

// RouteDNS runs `cloudflared tunnel route dns <name> <hostname>`.
func RouteDNS(bin, tunnelName, hostname string) error {
	if bin == "" {
		bin = "cloudflared"
	}
	out, err := exec.Command(bin, "tunnel", "route", "dns", tunnelName, hostname).CombinedOutput()
	if err != nil {
		return fmt.Errorf("routing DNS for %s -> %s: %w\n%s", tunnelName, hostname, err, string(out))
	}
	return nil
}

// StoreTunnelToken stores the cloudflared tunnel token in the macOS Keychain.
func StoreTunnelToken(tunnelName string) error {
	out, err := exec.Command("cloudflared", "tunnel", "token", tunnelName).Output()
	if err != nil {
		return fmt.Errorf("getting tunnel token: %w", err)
	}
	token := strings.TrimSpace(string(out))
	return keychain.Store(keychain.CFTokenAccount+"-"+tunnelName, []byte(token))
}

// LoadTunnelToken retrieves a tunnel token from the Keychain.
func LoadTunnelToken(tunnelName string) (string, error) {
	data, err := keychain.Load(keychain.CFTokenAccount + "-" + tunnelName)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Start launches the cloudflared tunnel in a goroutine.
// The tunnel runs until ctx is cancelled or the process exits.
func (m *Manager) Start(ctx context.Context) error {
	bin := m.cfg.CloudflaredBin

	if err := ValidateBinary(bin); err != nil {
		return err
	}

	tunnelCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// Build args: `cloudflared tunnel --url <local> run <name>`
	args := []string{
		"tunnel",
		"--url", m.cfg.LocalURL,
		"run", m.cfg.TunnelName,
	}

	m.logger.Info().
		Str("tunnel", m.cfg.TunnelName).
		Str("url", m.cfg.LocalURL).
		Str("hostname", m.cfg.Hostname).
		Msg("starting cloudflare tunnel")

	m.audit.MustLog(audit.EventTunnelStart, "cloudflare tunnel starting", map[string]string{
		"tunnel":   m.cfg.TunnelName,
		"hostname": m.cfg.Hostname,
	})

	m.cmd = exec.CommandContext(tunnelCtx, bin, args...)

	// Stream cloudflared logs to our logger.
	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("getting cloudflared stderr: %w", err)
	}

	if err := m.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("starting cloudflared: %w", err)
	}

	// Log cloudflared output.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			m.logger.Debug().Str("cloudflared", scanner.Text()).Msg("tunnel log")
		}
	}()

	// Watch for process exit.
	go func() {
		if err := m.cmd.Wait(); err != nil {
			if tunnelCtx.Err() == nil {
				m.logger.Error().Err(err).Msg("cloudflared exited unexpectedly")
			}
		}
		m.audit.MustLog(audit.EventTunnelStop, "cloudflare tunnel stopped", map[string]string{
			"tunnel": m.cfg.TunnelName,
		})
	}()

	// Give tunnel a moment to start before returning.
	time.Sleep(2 * time.Second)
	return nil
}

// Stop terminates the cloudflared process.
func (m *Manager) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Kill()
	}
	return nil
}

// ValidateTunnelReachable checks that the tunnel hostname is accessible.
func ValidateTunnelReachable(hostname string, timeout time.Duration) error {
	// Simple check: attempt TCP connect to port 443 of the hostname.
	// A full HTTP check would require TLS validation.
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := exec.Command("curl", "-sf", "--max-time", "5",
			"https://"+hostname+"/health").Output()
		if err == nil {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("tunnel at %s not reachable after %s", hostname, timeout)
}
