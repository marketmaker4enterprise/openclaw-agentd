// Package auth handles signed-token bearer authentication for the local agent API.
package auth

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/burmaster/openclaw-agentd/internal/crypto"
)

// TokenValidator validates signed bearer tokens presented by callers.
// A token is a serialised SignedRequest with an empty payload; the bearer
// must sign it with the caller's Ed25519 private key.
type TokenValidator struct {
	allowedPeers map[string]ed25519.PublicKey // agentID -> pubkey
	mu           sync.RWMutex

	// Simple nonce replay cache: nonce -> expiry time.
	nonces map[string]time.Time
	nonceMu sync.Mutex
}

// NewTokenValidator creates a validator with a pre-populated peer allowlist.
// peers maps agent-id to hex-encoded public key.
func NewTokenValidator(peers map[string]string) (*TokenValidator, error) {
	tv := &TokenValidator{
		allowedPeers: make(map[string]ed25519.PublicKey),
		nonces:       make(map[string]time.Time),
	}
	for id, pubHex := range peers {
		pub, err := crypto.PublicKeyFromHex(pubHex)
		if err != nil {
			return nil, fmt.Errorf("invalid public key for peer %s: %w", id, err)
		}
		tv.allowedPeers[id] = pub
	}
	return tv, nil
}

// AddPeer adds or updates a peer in the allowlist.
func (tv *TokenValidator) AddPeer(agentID, pubHex string) error {
	pub, err := crypto.PublicKeyFromHex(pubHex)
	if err != nil {
		return err
	}
	tv.mu.Lock()
	defer tv.mu.Unlock()
	tv.allowedPeers[agentID] = pub
	return nil
}

// RemovePeer removes a peer from the allowlist.
func (tv *TokenValidator) RemovePeer(agentID string) {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	delete(tv.allowedPeers, agentID)
}

// Validate checks a Bearer token from an Authorization header.
// The token must be a JSON-encoded SignedRequest.
// Returns the agentID of the authenticated peer or an error.
func (tv *TokenValidator) Validate(authHeader string) (string, error) {
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return "", fmt.Errorf("missing Bearer scheme")
	}

	var req crypto.SignedRequest
	if err := json.Unmarshal([]byte(token), &req); err != nil {
		return "", fmt.Errorf("invalid token format: %w", err)
	}

	tv.mu.RLock()
	defer tv.mu.RUnlock()

	for agentID, pub := range tv.allowedPeers {
		if err := crypto.Verify(pub, &req, tv.nonceUsed); err == nil {
			tv.recordNonce(req.Nonce)
			return agentID, nil
		}
	}
	return "", fmt.Errorf("no matching peer or invalid signature")
}

// nonceUsed returns true if nonce has been seen (replay protection).
func (tv *TokenValidator) nonceUsed(nonce string) bool {
	tv.nonceMu.Lock()
	defer tv.nonceMu.Unlock()
	tv.gcNonces()
	_, ok := tv.nonces[nonce]
	return ok
}

// recordNonce stores a nonce for future replay detection.
func (tv *TokenValidator) recordNonce(nonce string) {
	tv.nonceMu.Lock()
	defer tv.nonceMu.Unlock()
	expiry := time.Now().Add(crypto.ReplayWindowSeconds * time.Second)
	tv.nonces[nonce] = expiry
}

// gcNonces removes expired nonces. Must be called with nonceMu held.
func (tv *TokenValidator) gcNonces() {
	now := time.Now()
	for n, exp := range tv.nonces {
		if now.After(exp) {
			delete(tv.nonces, n)
		}
	}
}

// Middleware returns an HTTP middleware that enforces signed-token auth.
// If allowedPeerIDs is nil/empty, all peers with valid signatures are allowed.
func (tv *TokenValidator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "unauthorized: missing Authorization header", http.StatusUnauthorized)
			return
		}

		_, err := tv.Validate(authHeader)
		if err != nil {
			http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Simple HMAC-token fallback (for single-machine use) ---

// StaticToken is a time-and-nonce signed token using the agent's own Ed25519 key.
// Useful for curl-style local testing without a peer keypair.
func GenerateStaticToken(priv ed25519.PrivateKey, payload []byte) (string, error) {
	req, err := crypto.Sign(priv, payload)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	return "Bearer " + string(data), nil
}

// VerifyStaticToken verifies a token signed with the provided public key.
func VerifyStaticToken(pubHex string, authHeader string, seen func(string) bool) error {
	pub, err := crypto.PublicKeyFromHex(pubHex)
	if err != nil {
		return err
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return fmt.Errorf("missing Bearer scheme")
	}

	var req crypto.SignedRequest
	if err := json.Unmarshal([]byte(token), &req); err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	return crypto.Verify(pub, &req, seen)
}

// ConstantTimeEqual does a timing-safe string comparison.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// HexEncodePublicKey encodes an ed25519 public key to hex.
func HexEncodePublicKey(pub ed25519.PublicKey) string {
	return hex.EncodeToString(pub)
}
