class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.8"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.8.tar.gz"
  sha256 "1e9afe38935fa569ef401db5f2e727ee3d3505c46be0a30125feff77bea58c23"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.8", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
