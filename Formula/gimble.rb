class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.10"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.10.tar.gz"
  sha256 "6eba4cf81b09024cffa6a6da181b3700ab7ac76c5b96bfe6b3364fad920d314d"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.10", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
