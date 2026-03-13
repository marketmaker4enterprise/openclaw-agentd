// Package registry handles A2A challenge-response registration with agentboard.
package registry

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/burmaster/openclaw-agentd/internal/crypto"
)

const (
	challengeEndpoint = "/api/auth/challenge"
	respondEndpoint   = "/api/auth/respond"
	heartbeatEndpoint = "/api/agents/heartbeat"
	revokeEndpoint    = "/api/agents/revoke"
)

// ChallengeRequest asks the server to issue a challenge.
type ChallengeRequest struct {
	PublicKey string `json:"public_key"`
	Nonce     string `json:"nonce"`
	Timestamp int64  `json:"timestamp"`
}

// ChallengeResponse contains the server challenge bytes.
type ChallengeResponse struct {
	ChallengeID    string `json:"challenge_id"`
	ChallengeBytes string `json:"challenge_bytes"` // hex
}

// RespondRequest proves identity by signing the challenge.
type RespondRequest struct {
	ChallengeID    string `json:"challenge_id"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"` // hex(sign(challenge_bytes))
	Nonce          string `json:"nonce"`
	Timestamp      int64  `json:"timestamp"`
	AgentName      string `json:"agent_name"`
	PublicHostname string `json:"public_hostname"`
	Capabilities   []string `json:"capabilities"`
	Version        string `json:"version"`
}

// RespondResponse is the final registration acknowledgement.
type RespondResponse struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HeartbeatRequest is sent periodically to keep registration alive.
type HeartbeatRequest struct {
	AgentID   string `json:"agent_id"`
	Nonce     string `json:"nonce"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

// RevokeRequest signs a revocation.
type RevokeRequest struct {
	AgentID   string `json:"agent_id"`
	Nonce     string `json:"nonce"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

// Client talks to the agentboard registration API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	priv       ed25519.PrivateKey
	pub        ed25519.PublicKey
}

// NewClient creates a registry Client.
// If pub is nil it is derived from priv.
func NewClient(baseURL string, priv ed25519.PrivateKey, pub ed25519.PublicKey) *Client {
	if pub == nil && priv != nil {
		pub = priv.Public().(ed25519.PublicKey)
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		priv: priv,
		pub:  pub,
	}
}

// Register performs the full A2A challenge-response registration flow.
// Returns the assigned agent ID on success.
func (c *Client) Register(agentName, publicHostname string, capabilities []string, version string) (string, error) {
	// Step 1: Request a challenge.
	challenge, err := c.requestChallenge()
	if err != nil {
		return "", fmt.Errorf("requesting challenge: %w", err)
	}

	// Step 2: Sign the challenge and respond.
	agentID, err := c.respondChallenge(challenge, agentName, publicHostname, capabilities, version)
	if err != nil {
		return "", fmt.Errorf("responding to challenge: %w", err)
	}

	return agentID, nil
}

func (c *Client) requestChallenge() (*ChallengeResponse, error) {
	nonce, err := crypto.NewNonce()
	if err != nil {
		return nil, err
	}

	req := ChallengeRequest{
		PublicKey: hex.EncodeToString(c.pub),
		Nonce:     nonce,
		Timestamp: time.Now().Unix(),
	}

	var resp ChallengeResponse
	if err := c.post(challengeEndpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) respondChallenge(challenge *ChallengeResponse, agentName, hostname string, caps []string, version string) (string, error) {
	// Decode server challenge.
	challengeBytes, err := hex.DecodeString(challenge.ChallengeBytes)
	if err != nil {
		return "", fmt.Errorf("decoding challenge bytes: %w", err)
	}

	// Sign challenge bytes with our private key.
	sig := ed25519.Sign(c.priv, challengeBytes)

	nonce, err := crypto.NewNonce()
	if err != nil {
		return "", err
	}

	req := RespondRequest{
		ChallengeID:    challenge.ChallengeID,
		PublicKey:      hex.EncodeToString(c.pub),
		Signature:      hex.EncodeToString(sig),
		Nonce:          nonce,
		Timestamp:      time.Now().Unix(),
		AgentName:      agentName,
		PublicHostname: hostname,
		Capabilities:   caps,
		Version:        version,
	}

	var resp RespondResponse
	if err := c.post(respondEndpoint, req, &resp); err != nil {
		return "", err
	}

	if resp.Status != "ok" && resp.Status != "registered" && resp.AgentID == "" {
		return "", fmt.Errorf("registration failed: %s", resp.Message)
	}
	return resp.AgentID, nil
}

// Heartbeat sends a keepalive to the registry.
func (c *Client) Heartbeat(agentID string) error {
	nonce, err := crypto.NewNonce()
	if err != nil {
		return err
	}
	ts := time.Now().Unix()

	msg := []byte(fmt.Sprintf("%s|%s|%d", agentID, nonce, ts))
	sig := ed25519.Sign(c.priv, msg)

	req := HeartbeatRequest{
		AgentID:   agentID,
		Nonce:     nonce,
		Timestamp: ts,
		Signature: hex.EncodeToString(sig),
	}

	var resp map[string]interface{}
	return c.post(heartbeatEndpoint, req, &resp)
}

// Revoke deregisters the agent from the registry.
func (c *Client) Revoke(agentID string) error {
	nonce, err := crypto.NewNonce()
	if err != nil {
		return err
	}
	ts := time.Now().Unix()

	msg := []byte(fmt.Sprintf("revoke|%s|%s|%d", agentID, nonce, ts))
	sig := ed25519.Sign(c.priv, msg)

	req := RevokeRequest{
		AgentID:   agentID,
		Nonce:     nonce,
		Timestamp: ts,
		Signature: hex.EncodeToString(sig),
	}

	var resp map[string]interface{}
	return c.post(revokeEndpoint, req, &resp)
}

// post sends a JSON POST request and decodes the JSON response.
func (c *Client) post(endpoint string, body, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling request: %w", err)
	}

	url := c.baseURL + endpoint
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respData))
	}

	if out != nil {
		if err := json.Unmarshal(respData, out); err != nil {
			return fmt.Errorf("decoding response: %w (body: %s)", err, string(respData))
		}
	}
	return nil
}
