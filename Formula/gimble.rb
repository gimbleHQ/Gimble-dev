class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.10"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.10.tar.gz"
  sha256 "1c1e02966e969d70b770a6d3714b947a473bfbe4f16debdf2bfd5974c63757be"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.10", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
