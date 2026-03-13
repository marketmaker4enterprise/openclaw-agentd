# Threat Model — openclaw-agentd

## Assets

| Asset | Sensitivity | Location |
|-------|-------------|----------|
| Ed25519 private key | Critical | macOS Keychain |
| Cloudflare tunnel credentials | High | macOS Keychain |
| agentboard JWT | High | Memory only (never persisted) |
| config.yaml | Medium | ~/.config/openclaw-agentd/ |
| Audit log | Medium | ~/.local/share/openclaw-agentd/ |
| Local OpenClaw agent API | High | 127.0.0.1 only |

## Threats and Controls

### T1 — Malicious agent impersonation
**Scenario:** Attacker registers a fake agent claiming to be a trusted peer.  
**Control:** Every request signed with Ed25519. `allowed_peer_ids` allowlist enforced in proxy. Signatures verified before forwarding.

### T2 — Replay attacks
**Scenario:** Attacker captures a valid signed request and replays it.  
**Controls:**
- Nonce embedded in every signed request (16 random bytes)
- Timestamp ±5 minute window enforced
- Nonce tracking in memory (cleared on restart — acceptable for 5-min window)
- Agentboard also checks nonce uniqueness server-side

### T3 — MITM between proxy and tunnel / tunnel and registry
**Controls:**
- Cloudflare Tunnel uses TLS 1.3 with certificate pinning
- All registry calls use HTTPS; server cert validated
- No plain HTTP allowed

### T4 — Accidental public exposure of local service
**Controls:**
- Proxy binds to `127.0.0.1` only by default
- `0.0.0.0` requires explicit `--bind-all` flag + interactive confirmation with clear warning
- `init` shows exact hostname before creating tunnel, requires `yes` confirmation

### T5 — Stolen API tokens / private keys
**Controls:**
- Private key stored in macOS Keychain (hardware-backed on Apple Silicon)
- JWT lives in memory only; never written to disk or logged
- `rotate-keys` allows fast remediation if key is suspected compromised
- Revoke command deregisters agent immediately

### T6 — DNS misconfiguration
**Controls:**
- `expose` validates DNS resolution of the public hostname before registration
- If DNS does not resolve to the Cloudflare edge, registration is aborted

### T7 — Unauthorized registration (fake agent at agentboard)
**Controls:**
- Registration payload signed with Ed25519 private key
- Agentboard verifies signature against public key submitted at registration
- Nonce + timestamp prevent replayed registration requests

### T8 — Local privilege escalation
**Controls:**
- Agent runs as the current user; no setuid, no root requirement
- Config files owned by current user (0600)
- Audit log append-only

### T9 — Supply chain tampering
**Controls:**
- Homebrew formula pins specific release artifact with SHA256 checksum
- No curl-pipe-to-shell in install flow
- Go modules with pinned versions and checksums (go.sum)
- Reproducible builds via Makefile

### T10 — Malicious config injection via CLI flags or config file
**Controls:**
- Config schema validated on load with strict allowlist of known fields
- Unknown fields rejected (not silently ignored)
- CLI flags sanitized via Cobra's type system

### T11 — SSRF via registration metadata
**Controls:**
- `public_endpoint` URL validated to HTTPS scheme + DNS resolution before submission
- No server-side URL fetching of agent-provided URLs in agentboard registration path

### T12 — Abuse of open endpoints by anonymous internet clients
**Controls:**
- All write endpoints require JWT Bearer
- Cloudflare Tunnel can be combined with Cloudflare Access (IP/identity policy) for additional layer
- Rate limiting on all endpoints (token bucket, configurable)

## Accepted Risks

| Risk | Rationale |
|------|-----------|
| Nonce store lost on restart | 5-min window is narrow; production deployments may add persistent nonce store |
| Single-machine deployment | No HA; agent goes offline if machine restarts |
| Cloudflare as trust anchor | Cloudflare can in theory MITM tunnel traffic; acceptable for agent-to-agent use case |
