class OpenclawAgentd < Formula
  desc "Secure Homebrew CLI to expose a local OpenClaw agent via Cloudflare Tunnel and register with agentboard.burmaster.com"
  homepage "https://agentboard.burmaster.com"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-arm64.tar.gz"
      sha256 "43440ed6825f1390c383a30d733f20165b8c3e0e95beb017d2692c3258496329"
    else
      url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-amd64.tar.gz"
      sha256 "a58d95ad545bbc7b8feda0807c3cf822d7ba875c47aaeb81ba48613746a8cc2e"
    end
  end

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
