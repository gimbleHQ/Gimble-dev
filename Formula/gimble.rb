class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/gimbleHQ/Gimble-dev"
  version "1.0.0"
  url "https://github.com/gimbleHQ/Gimble-dev/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "56fe12d55aea0903851d04b797354562e5aca9d79b3c5534f105c3f63c424ee2"
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
