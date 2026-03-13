// Package keychain wraps macOS Keychain Services for secret storage.
// On non-darwin platforms it falls back to a mode-600 file in the config dir.
package keychain

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

const (
	// ServiceName is the Keychain service identifier.
	ServiceName = "openclaw-agentd"
	// PrivKeyAccount is the Keychain account name for the Ed25519 private key.
	PrivKeyAccount = "ed25519-private-key"
	// CFTokenAccount is the Keychain account name for the Cloudflare tunnel token.
	CFTokenAccount = "cloudflare-tunnel-token"
)

// Store saves a secret. On macOS this uses the system Keychain;
// on other platforms it writes a mode-600 file (for dev/testing only).
func Store(account string, secret []byte) error {
	if runtime.GOOS == "darwin" {
		return darwinStore(ServiceName, account, secret)
	}
	return fileStore(account, secret)
}

// Load retrieves a secret.
func Load(account string) ([]byte, error) {
	if runtime.GOOS == "darwin" {
		return darwinLoad(ServiceName, account)
	}
	return fileLoad(account)
}

// Delete removes a secret.
func Delete(account string) error {
	if runtime.GOOS == "darwin" {
		return darwinDelete(ServiceName, account)
	}
	return fileDelete(account)
}

// --- File-based fallback (non-macOS) ---

func secretFilePath(account string) (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	// Sanitize account name for use as filename.
	safe := filepath.Base(account) // prevent path traversal
	return filepath.Join(dir, ".secret."+safe), nil
}

func fileStore(account string, secret []byte) error {
	path, err := secretFilePath(account)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating secret dir: %w", err)
	}
	return os.WriteFile(path, secret, 0600)
}

func fileLoad(account string) ([]byte, error) {
	path, err := secretFilePath(account)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("secret not found for account %q", account)
	}
	return data, err
}

func fileDelete(account string) error {
	path, err := secretFilePath(account)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
