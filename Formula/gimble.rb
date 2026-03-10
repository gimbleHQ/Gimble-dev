class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.6"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.6.tar.gz"
  sha256 "7014158e35c4758b1bd12214e1362eadcfabc978c3d9f708c84132d07c888d53"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.6", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
