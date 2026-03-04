class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/gimble-dev/gimble"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/gimble-dev/gimble/releases/download/v#{version}/gimble-darwin-arm64"
      sha256 "REPLACE_WITH_REAL_SHA256"
    else
      url "https://github.com/gimble-dev/gimble/releases/download/v#{version}/gimble-darwin-amd64"
      sha256 "REPLACE_WITH_REAL_SHA256"
    end
  end

  def install
    bin.install Dir["*"][0] => "gimble"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
