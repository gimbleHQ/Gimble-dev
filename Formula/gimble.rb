class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.5"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.5.tar.gz"
  sha256 "c2df46b8979432507cf4a434ee8d2b52bd340ceff348902d0f5304f0e03502ba"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.5", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
