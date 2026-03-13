package tests

import (
	"testing"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	// BindAddress may include port (e.g. "127.0.0.1:7878") — must start with 127.0.0.1
	if len(cfg.BindAddress) < 9 || cfg.BindAddress[:9] != "127.0.0.1" {
		t.Fatalf("expected default bind address to start with 127.0.0.1, got %s", cfg.BindAddress)
	}
	if cfg.AuthMode != "signed-token" {
		t.Fatalf("expected default auth mode signed-token, got %s", cfg.AuthMode)
	}
	if cfg.ConsentGeolocation {
		t.Fatal("geolocation consent must default to false")
	}
	if cfg.RateLimits.RequestsPerMinute == 0 {
		t.Fatal("rate limit must have a non-zero default")
	}
}

func TestDefaultBindAddressIsLocalhost(t *testing.T) {
	cfg := config.DefaultConfig()
	if len(cfg.BindAddress) < 9 || cfg.BindAddress[:9] != "127.0.0.1" {
		t.Fatalf("default bind address must start with 127.0.0.1, got %s", cfg.BindAddress)
	}
}

func TestRegistrationURLDefaultSet(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.RegistrationAPIURL == "" {
		t.Fatal("default registration API URL must not be empty")
	}
}

func TestDefaultRateLimitNonZero(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.RateLimits.RequestsPerMinute <= 0 {
		t.Fatal("default rate limit must be > 0")
	}
}

func TestDefaultHeartbeatInterval(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.HeartbeatInterval == 0 {
		t.Fatal("default heartbeat interval must not be zero")
	}
}
