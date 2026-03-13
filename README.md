# openclaw-agentd

Secure Homebrew CLI to expose a local [OpenClaw](https://agora.burmaster.com) agent via Cloudflare Tunnel and register it with [AgentBoard](https://agentboard.burmaster.com) for agent-to-agent discovery.

## What it does

```
brew install openclaw-agentd
openclaw-agentd init
openclaw-agentd expose
```

1. Generates an Ed25519 identity keypair (stored in macOS Keychain)
2. Creates a Cloudflare Tunnel — no open firewall ports, no port forwarding
3. Publishes a public hostname (`agent-<id>.yourdomain.com`)
4. Registers the agent with agentboard.burmaster.com using A2A signed authentication
5. Sends periodic heartbeats to stay discoverable

Other agents can now find and call your agent via [AgentDNS](https://agentboard.burmaster.com/docs).

---

## Install

**Prerequisites**
- macOS 13+ (Ventura or later)
- [Homebrew](https://brew.sh)
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/) (`brew install cloudflared`)
- A local OpenClaw agent running at `http://127.0.0.1:18789` (or configure a different URL)

```bash
brew install openclaw-agentd
```

---

## Quick start

```bash
# 1. Initialize — generates keys, walks through Cloudflare auth, registers
openclaw-agentd init

# 2. Check everything is working
openclaw-agentd status

# 3. Expose publicly (creates tunnel if not already running)
openclaw-agentd expose
```

---

## Commands

| Command | Description |
|---------|-------------|
| `init` | First-run setup: keygen, Cloudflare auth, validate local agent, confirm, register |
| `login` | Re-authenticate with agentboard (refresh JWT) |
| `configure` | Edit config interactively |
| `expose` | Create/attach Cloudflare Tunnel, verify, re-register |
| `register` | Register or update registration with agentboard |
| `status` | Show local API, tunnel, registration, key fingerprint, heartbeat |
| `rotate-keys` | Rotate Ed25519 signing keypair and update registration |
| `revoke` | Deregister from agentboard, optionally tear down tunnel |
| `stop` | Gracefully stop the daemon |
| `doctor` | Diagnose common issues (deps, config, connectivity) |
| `uninstall` | Full teardown: remove keys from Keychain, delete config, stop daemon |

---

## Status output

```
openclaw-agentd status

  Local API         ✓ reachable (http://127.0.0.1:18789)
  Public endpoint   ✓ healthy (https://agent-abc123.example.com)
  Cloudflare tunnel ✓ connected (openclaw-agent-abc123)
  Registration      ✓ registered (agentboard.burmaster.com)
  Auth mode         signed-token
  Key fingerprint   SHA256:xK3m...
  Last heartbeat    42s ago
  Warnings          none
```

---

## Configuration

Config file: `~/.config/openclaw-agentd/config.yaml`

```yaml
local_agent_url: "http://127.0.0.1:18789"
bind_address: "127.0.0.1"          # NEVER change to 0.0.0.0
public_hostname: ""                 # set by `expose`
agent_name: "My Agent"
capabilities: [research, writing]
allowed_peer_ids: []               # empty = no peer calls (safest default)
heartbeat_interval: "5m"
log_level: "info"
consent_geolocation: false
```

See [`examples/config.yaml`](examples/config.yaml) for all options with comments.

**Never put secrets in config.yaml.** Private key and Cloudflare credentials are stored in macOS Keychain.

---

## Security

- **Ed25519 signing** on all registration requests + agent-to-agent calls
- **Replay protection**: nonce + ±5 minute timestamp window
- **macOS Keychain** for private key (hardware-backed on Apple Silicon)
- **Localhost-only** bind by default; public exposure requires explicit confirmation
- **Cloudflare Tunnel** as the only public ingress — no open ports
- **Audit log** at `~/.local/share/openclaw-agentd/audit.log`

See [`docs/threat-model.md`](docs/threat-model.md) for full threat model and [`docs/hardening-checklist.md`](docs/hardening-checklist.md) for operational security checklist.

---

## Registration API

`openclaw-agentd` uses the [AgentBoard A2A protocol](https://agentboard.burmaster.com/docs):

```
POST /api/auth/register    # register agent (idempotent)
POST /api/auth/challenge   # get challenge
POST /api/auth/respond     # submit Ed25519 proof, receive JWT
POST /api/agents/heartbeat # liveness signal
POST /api/agents/revoke    # deregister
GET  /api/agents/{id}      # lookup agent in AgentDNS
```

---

## Build from source

```bash
git clone https://github.com/marketmaker4enterprise/openclaw-agentd
cd openclaw-agentd
make build
./dist/openclaw-agentd --version
```

Run tests:
```bash
make test
```

Build macOS release artifacts:
```bash
make release
# Updates dist/checksums.txt — paste SHA256 values into Formula/openclaw-agentd.rb
```

---

## Publish to Homebrew

1. `make release` — builds + checksums
2. Upload `dist/*.tar.gz` to GitHub Releases
3. Update `Formula/openclaw-agentd.rb` with the SHA256 values from `dist/checksums.txt`
4. Submit formula to your Homebrew tap or `homebrew-core`

---

## Uninstall

```bash
openclaw-agentd uninstall   # removes keys, config, stops daemon, deregisters
brew uninstall openclaw-agentd
```

---

## License

MIT — see [LICENSE](LICENSE)
