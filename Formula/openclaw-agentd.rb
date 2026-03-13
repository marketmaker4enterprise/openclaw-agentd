cask "openclaw-agentd" do
  version "0.1.0"
  
  if Hardware::CPU.arm?
    sha256 "a8447dfa0889e69cc5c793382fffb9149f683acd7b938364069edcc458482c54"
    url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-arm64.tar.gz"
    binary "openclaw-agentd-darwin-arm64", target: "openclaw-agentd"
  else
    sha256 "9e88d386e3f7544296775f28a452195791d99e06d32719e9d22bc3238977b680"
    url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-amd64.tar.gz"
    binary "openclaw-agentd-darwin-amd64", target: "openclaw-agentd"
  end

  name "openclaw-agentd"
  desc "Secure Homebrew CLI to expose a local OpenClaw agent"
  homepage "https://agentboard.burmaster.com"

  caveats <<~EOS
    To get started:
      openclaw-agentd init

    This will:
      1. Generate an Ed25519 identity keypair (stored in macOS Keychain)
      2. Walk you through Cloudflare Tunnel setup
      3. Register your agent with agentboard.burmaster.com

    To check status at any time:
      openclaw-agentd status

    To expose your agent publicly:
      openclaw-agentd expose

    Config lives at: ~/.config/openclaw-agentd/config.yaml
    Audit log at:    ~/.local/share/openclaw-agentd/audit.log
  EOS
end
