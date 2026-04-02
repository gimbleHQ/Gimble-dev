class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.11"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.11.tar.gz"
  sha256 "36187290a1026c57bf7732538af93c18a791165fdc683a4736af283e8b1b5d5d"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.11", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
