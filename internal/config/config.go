// Package config manages openclaw-agentd configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir        = ".config/openclaw-agentd"
	DefaultConfigFile       = "config.yaml"
	DefaultBindAddress      = "127.0.0.1:7878"
	DefaultRegistrationAPI  = "https://agentboard.burmaster.com"
	DefaultHeartbeatInterval = 60 * time.Second
	DefaultLogLevel         = "info"
	DefaultAuthMode         = "signed-token"
)

// RateLimitConfig controls per-endpoint rate limiting.
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute" mapstructure:"requests_per_minute"`
	BurstSize         int `yaml:"burst_size" mapstructure:"burst_size"`
}

// Config is the full agent configuration.
type Config struct {
	// Networking
	LocalAgentURL  string `yaml:"local_agent_url" mapstructure:"local_agent_url"`
	BindAddress    string `yaml:"bind_address" mapstructure:"bind_address"`
	PublicHostname string `yaml:"public_hostname" mapstructure:"public_hostname"`

	// Cloudflare
	CloudflareTunnelName string `yaml:"cloudflare_tunnel_name" mapstructure:"cloudflare_tunnel_name"`

	// Registration
	RegistrationAPIURL string `yaml:"registration_api_url" mapstructure:"registration_api_url"`
	AgentName          string `yaml:"agent_name" mapstructure:"agent_name"`
	AgentID            string `yaml:"agent_id" mapstructure:"agent_id"`

	// Capabilities advertised to peers.
	Capabilities []string `yaml:"capabilities" mapstructure:"capabilities"`

	// Allowlist of peer agent IDs permitted to call this agent.
	AllowedPeerIDs []string `yaml:"allowed_peer_ids" mapstructure:"allowed_peer_ids"`

	// Auth
	AuthMode string `yaml:"auth_mode" mapstructure:"auth_mode"`

	// Keys
	PublicKey string `yaml:"public_key" mapstructure:"public_key"` // hex-encoded Ed25519 public key

	// Rate limits
	RateLimits RateLimitConfig `yaml:"rate_limits" mapstructure:"rate_limits"`

	// Observability
	LogLevel string `yaml:"log_level" mapstructure:"log_level"`

	// Privacy
	ConsentGeolocation bool `yaml:"consent_geolocation" mapstructure:"consent_geolocation"`

	// Heartbeat
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"`
}

// DefaultConfig returns a Config pre-filled with sane defaults.
func DefaultConfig() *Config {
	return &Config{
		BindAddress:        DefaultBindAddress,
		LocalAgentURL:      "http://" + DefaultBindAddress,
		RegistrationAPIURL: DefaultRegistrationAPI,
		AuthMode:           DefaultAuthMode,
		LogLevel:           DefaultLogLevel,
		HeartbeatInterval:  DefaultHeartbeatInterval,
		RateLimits: RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         10,
		},
		Capabilities:   []string{"execute", "query"},
		AllowedPeerIDs: []string{},
	}
}

// ConfigDir returns the OS-specific config directory.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir), nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultConfigFile), nil
}

// Load reads config from disk. Returns defaults if file doesn't exist yet.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return cfg, nil
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return cfg, nil
}

// Save writes cfg to disk, creating directories as needed.
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	path := filepath.Join(dir, DefaultConfigFile)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// AuditLogPath returns the path for the audit log file.
func AuditLogPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit.log"), nil
}

// PIDFilePath returns the path for the agent PID file.
func PIDFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agent.pid"), nil
}
