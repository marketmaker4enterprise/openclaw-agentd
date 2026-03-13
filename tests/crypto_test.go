package tests

import (
	"testing"
	"time"

	"github.com/burmaster/openclaw-agentd/internal/crypto"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if kp.PrivateKey == nil || kp.PublicKey == nil {
		t.Fatal("keypair has nil keys")
	}
	if len(kp.PublicKey) != 32 {
		t.Fatalf("expected 32-byte public key, got %d", len(kp.PublicKey))
	}
}

func TestSignAndVerify(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	payload := []byte(`{"agent_id":"test-agent-01","capability":"research"}`)

	signed, err := crypto.Sign(kp.PrivateKey, payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if signed.Signature == "" {
		t.Fatal("signature is empty")
	}
	if signed.Nonce == "" {
		t.Fatal("nonce is empty")
	}
	if signed.Timestamp == 0 {
		t.Fatal("timestamp is zero")
	}
}

func TestReplayProtection(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	payload := []byte(`{"agent_id":"test-agent-01"}`)

	signed, err := crypto.Sign(kp.PrivateKey, payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	seenNonces := make(map[string]bool)
	seenFn := func(nonce string) bool {
		if seenNonces[nonce] {
			return true // already seen = replay
		}
		seenNonces[nonce] = true
		return false
	}

	// First verify — should pass
	if err := crypto.Verify(kp.PublicKey, signed, seenFn); err != nil {
		t.Fatalf("Verify failed on first call: %v", err)
	}

	// Second verify with same nonce — should fail (replay)
	if err := crypto.Verify(kp.PublicKey, signed, seenFn); err == nil {
		t.Fatal("expected replay protection to reject duplicate nonce")
	}
}

func TestExpiredTimestamp(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	payload := []byte(`{"agent_id":"test-agent-01"}`)

	signed, err := crypto.Sign(kp.PrivateKey, payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Wind the timestamp back 10 minutes
	signed.Timestamp = time.Now().Add(-10 * time.Minute).Unix()

	seenFn := func(string) bool { return false }
	if err := crypto.Verify(kp.PublicKey, signed, seenFn); err == nil {
		t.Fatal("expected timestamp rejection for expired payload")
	}
}

func TestWrongKeyRejected(t *testing.T) {
	kp1, _ := crypto.GenerateKeyPair()
	kp2, _ := crypto.GenerateKeyPair()

	payload := []byte(`{"agent_id":"test-agent-01"}`)
	signed, _ := crypto.Sign(kp1.PrivateKey, payload)

	seenFn := func(string) bool { return false }
	if err := crypto.Verify(kp2.PublicKey, signed, seenFn); err == nil {
		t.Fatal("expected signature verification to fail with wrong public key")
	}
}

func TestNonceUniqueness(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	payload := []byte(`{"agent_id":"test-agent-01"}`)

	nonces := make(map[string]bool)
	for i := 0; i < 100; i++ {
		signed, err := crypto.Sign(kp.PrivateKey, payload)
		if err != nil {
			t.Fatalf("Sign failed on iteration %d: %v", i, err)
		}
		if nonces[signed.Nonce] {
			t.Fatalf("duplicate nonce on iteration %d", i)
		}
		nonces[signed.Nonce] = true
	}
}

func TestPublicKeyHex(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	hex := kp.PublicKeyHex()
	if len(hex) != 64 { // 32 bytes × 2 hex chars
		t.Fatalf("expected 64-char hex, got %d", len(hex))
	}

	restored, err := crypto.PublicKeyFromHex(hex)
	if err != nil {
		t.Fatalf("PublicKeyFromHex failed: %v", err)
	}
	if string(restored) != string(kp.PublicKey) {
		t.Fatal("restored public key does not match original")
	}
}
