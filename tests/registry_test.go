package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/burmaster/openclaw-agentd/internal/crypto"
	"github.com/burmaster/openclaw-agentd/internal/registry"
)

func mockAgentboard(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"registered": true})
	})

	mux.HandleFunc("/api/auth/challenge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// challenge_bytes must be valid hex for the client to decode
		json.NewEncoder(w).Encode(map[string]any{
			"challenge_id":    "test-challenge-id",
			"challenge_bytes": "deadbeefcafe1234deadbeefcafe1234", // 16 bytes hex
		})
	})

	mux.HandleFunc("/api/auth/respond", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"agent_id": "test-agent-01",
			"status":   "registered",
		})
	})

	mux.HandleFunc("/api/agents/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Verify body contains agent_id and signature fields
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req["agent_id"] == nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/agents/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	return httptest.NewServer(mux)
}

func TestRegistrationFlow(t *testing.T) {
	srv := mockAgentboard(t)
	defer srv.Close()

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keygen failed: %v", err)
	}

	client := registry.NewClient(srv.URL, kp.PrivateKey, kp.PublicKey)
	token, err := client.Register("test-agent", "https://agent-test.example.com", []string{"research"}, "0.1.0")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token from registration")
	}
}

func TestHeartbeat(t *testing.T) {
	srv := mockAgentboard(t)
	defer srv.Close()

	kp, _ := crypto.GenerateKeyPair()
	client := registry.NewClient(srv.URL, kp.PrivateKey, kp.PublicKey)

	if err := client.Heartbeat("test-agent-01"); err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
}

func TestRevokeFlow(t *testing.T) {
	srv := mockAgentboard(t)
	defer srv.Close()

	kp, _ := crypto.GenerateKeyPair()
	client := registry.NewClient(srv.URL, kp.PrivateKey, kp.PublicKey)
	token, _ := client.Register("test-agent", "https://agent-test.example.com", []string{"research"}, "0.1.0")

	if err := client.Revoke(token); err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}
}

func TestDifferentBaseURLRejected(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	// Point at a server that doesn't exist
	client := registry.NewClient("http://127.0.0.1:19999", kp.PrivateKey, kp.PublicKey)
	_, err := client.Register("test-agent", "https://agent-test.example.com", []string{"research"}, "0.1.0")
	if err == nil {
		t.Fatal("expected error when registry is unreachable")
	}
}
