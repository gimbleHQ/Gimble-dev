class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.11"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.11.tar.gz"
  sha256 "42a3fcc97f5fe0003b2aba5bc2626194ed3836b2e492c80c93a054131e74285f"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.11", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
