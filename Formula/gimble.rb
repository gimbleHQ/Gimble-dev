class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.6"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.6.tar.gz"
  sha256 "f6522a6ca9dc24c1f77ef52b32443ec47b1a6f7f2a62d8643ab80d3d3ebb44ac"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.6", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
