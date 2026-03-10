class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.7"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.7.tar.gz"
  sha256 "90e9815d6a9355d41980f9a5e34ad62a94ce5fa34d64166b985306c13b19799c"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.7", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
