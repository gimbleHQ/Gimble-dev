class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.8"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.8.tar.gz"
  sha256 "063975048cb789c7bbacf34527e2b53c0b05e4728d5796c5c551385662eef7e7"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.8", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
