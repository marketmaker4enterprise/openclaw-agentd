// Package crypto provides Ed25519 key generation, signing, and verification
// with replay-attack protection.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

const (
	// ReplayWindowSeconds is the maximum age of a signed request we accept.
	ReplayWindowSeconds = 300 // 5 minutes
	nonceBytes          = 16
)

// KeyPair holds an Ed25519 signing keypair.
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// GenerateKeyPair creates a fresh Ed25519 keypair.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ed25519 keypair: %w", err)
	}
	return &KeyPair{PrivateKey: priv, PublicKey: pub}, nil
}

// PublicKeyHex returns the public key as a hex string.
func (kp *KeyPair) PublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey)
}

// PrivateKeyBytes returns the raw private key bytes for storage.
func (kp *KeyPair) PrivateKeyBytes() []byte {
	return []byte(kp.PrivateKey)
}

// PublicKeyFromHex decodes a hex-encoded public key.
func PublicKeyFromHex(s string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decoding public key hex: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// PrivateKeyFromBytes restores a private key from raw bytes.
func PrivateKeyFromBytes(b []byte) (ed25519.PrivateKey, error) {
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(b), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(b), nil
}

// SignedRequest represents a request with replay-protection fields.
type SignedRequest struct {
	Nonce     string `json:"nonce"`
	Timestamp int64  `json:"timestamp"` // Unix seconds
	Payload   []byte `json:"payload,omitempty"`
	Signature string `json:"signature"` // hex-encoded
}

// NewNonce generates a cryptographically random hex nonce.
func NewNonce() (string, error) {
	b := make([]byte, nonceBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// randomInt64 returns a cryptographically random int64 in [0, max).
func randomInt64(max int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

// Sign creates a SignedRequest for payload using priv.
func Sign(priv ed25519.PrivateKey, payload []byte) (*SignedRequest, error) {
	nonce, err := NewNonce()
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
	msg := buildSignMessage(nonce, ts, payload)
	sig := ed25519.Sign(priv, msg)

	return &SignedRequest{
		Nonce:     nonce,
		Timestamp: ts,
		Payload:   payload,
		Signature: hex.EncodeToString(sig),
	}, nil
}

// Verify checks the SignedRequest's signature and replay window.
// seen should be a non-nil function that returns true if nonce was already used.
func Verify(pub ed25519.PublicKey, req *SignedRequest, seen func(string) bool) error {
	// Check timestamp window.
	age := time.Now().Unix() - req.Timestamp
	if age < 0 {
		age = -age
	}
	if age > ReplayWindowSeconds {
		return fmt.Errorf("request timestamp outside replay window (age=%ds)", age)
	}

	// Check nonce replay.
	if seen != nil && seen(req.Nonce) {
		return fmt.Errorf("nonce already used (replay attack)")
	}

	// Verify signature.
	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	msg := buildSignMessage(req.Nonce, req.Timestamp, req.Payload)
	if !ed25519.Verify(pub, msg, sigBytes) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// buildSignMessage constructs the canonical message for signing.
// Format: nonce|timestamp|payload
func buildSignMessage(nonce string, ts int64, payload []byte) []byte {
	prefix := fmt.Sprintf("%s|%d|", nonce, ts)
	msg := make([]byte, len(prefix)+len(payload))
	copy(msg, []byte(prefix))
	copy(msg[len(prefix):], payload)
	return msg
}

// _ suppress unused import warning for randomInt64
var _ = randomInt64
