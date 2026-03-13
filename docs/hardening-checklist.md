# Hardening Checklist — openclaw-agentd

## Before First Run

- [ ] Run `openclaw-agentd init` — do NOT copy keys from another machine
- [ ] Verify macOS Keychain entry created: `Keychain Access → openclaw-agentd`
- [ ] Review `~/.config/openclaw-agentd/config.yaml` — confirm `bind_address: 127.0.0.1`
- [ ] Set `allowed_peer_ids` to the exact agent IDs you trust (empty = no peer calls)
- [ ] Set `rate_limits.requests_per_minute` appropriate for your use case
- [ ] Confirm `consent_geolocation: false` unless you explicitly want region in registry

## Network

- [ ] Confirm `0.0.0.0` is NOT in `bind_address`
- [ ] Confirm cloudflared is the only public ingress (no manual port forwards)
- [ ] Optionally configure Cloudflare Access policy on the public hostname for extra auth layer
- [ ] Verify `openclaw-agentd status` shows tunnel connected before sending any peer traffic

## Secrets

- [ ] Private key is in macOS Keychain — NOT in config.yaml or any file
- [ ] JWT token is not logged (check audit.log — it should not appear)
- [ ] Cloudflare credentials are in Keychain — NOT in ~/.cloudflared/ or config.yaml
- [ ] `OPENCLAW_AGENTD_*` env vars not set in shell profile (use Keychain instead)

## Ongoing

- [ ] Run `openclaw-agentd rotate-keys` every 90 days (or immediately if key suspected compromised)
- [ ] Review `~/.local/share/openclaw-agentd/audit.log` weekly for anomalies:
  - Unexpected registration events
  - Failed auth attempts from unknown agents
  - Key rotation events you did not initiate
- [ ] Keep `cloudflared` updated: `brew upgrade cloudflared`
- [ ] Keep `openclaw-agentd` updated: `brew upgrade openclaw-agentd`
- [ ] Verify agentboard registration is current: `openclaw-agentd status`

## If Compromised

1. `openclaw-agentd revoke` — immediately deregisters from agentboard
2. `openclaw-agentd rotate-keys` — generates new keypair, invalidates old
3. In Cloudflare dashboard: delete the tunnel manually if needed
4. Review audit log for timeline of unauthorized activity
5. Update `allowed_peer_ids` to remove any agents you no longer trust

## Uninstall / Cleanup

- [ ] `openclaw-agentd uninstall` — removes keys from Keychain, stops daemon, removes config
- [ ] `brew uninstall openclaw-agentd`
- [ ] Verify Keychain entry removed: `Keychain Access → search "openclaw-agentd"`
- [ ] Verify Cloudflare tunnel deleted in dashboard
- [ ] Verify agent no longer appears in agentboard DNS: `GET /api/agents/<your-agent-id>`
