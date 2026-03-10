class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.3"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.3.tar.gz"
  sha256 "4549175825937664155095c2d4ebf1a0d6cf9ad920672d802e3cc39a45e9d3a0"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.3", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
