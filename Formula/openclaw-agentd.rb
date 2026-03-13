class OpenclawAgentd < Formula
  desc "Secure Homebrew CLI to expose a local OpenClaw agent via Cloudflare Tunnel and register with agentboard.burmaster.com"
  homepage "https://agentboard.burmaster.com"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-arm64.tar.gz"
      # TODO: fill in after running `make release` and uploading to GitHub releases
      sha256 "a8447dfa0889e69cc5c793382fffb9149f683acd7b938364069edcc458482c54"
    else
      url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-amd64.tar.gz"
      sha256 "9e88d386e3f7544296775f28a452195791d99e06d32719e9d22bc3238977b680"
    end
  end

  depends_on :macos
  depends_on "cloudflared"

  def install
    if Hardware::CPU.arm?
      bin.install "openclaw-agentd-darwin-arm64" => "openclaw-agentd"
    else
      bin.install "openclaw-agentd-darwin-amd64" => "openclaw-agentd"
    end
  end

  def caveats
    <<~EOS
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

  test do
    system "#{bin}/openclaw-agentd", "--version"
  end
end
