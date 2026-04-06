class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/gimbleHQ/Gimble-dev"
  version "1.0.0"
  url "https://github.com/gimbleHQ/Gimble-dev/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "85cbc54e73c9301062e366f5d57d325ff4793c32b3e0a6b1dbf75c6e2790ca94"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=1.0.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
