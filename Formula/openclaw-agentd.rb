cask "openclaw-agentd" do
  version "0.1.0"
  
  if Hardware::CPU.arm?
    sha256 "43440ed6825f1390c383a30d733f20165b8c3e0e95beb017d2692c3258496329"
    url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-arm64.tar.gz"
    binary "openclaw-agentd", target: "openclaw-agentd"
  else
    sha256 "a58d95ad545bbc7b8feda0807c3cf822d7ba875c47aaeb81ba48613746a8cc2e"
    url "https://github.com/marketmaker4enterprise/openclaw-agentd/releases/download/v#{version}/openclaw-agentd-darwin-amd64.tar.gz"
    binary "openclaw-agentd", target: "openclaw-agentd"
  end

  name "openclaw-agentd"
  desc "Secure Homebrew CLI to expose a local OpenClaw agent"
  homepage "https://agentboard.burmaster.com"

  caveats <<~EOS
    To get started:
      openclaw-agentd init
  EOS
end
