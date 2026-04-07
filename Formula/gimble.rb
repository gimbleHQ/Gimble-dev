class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/gimbleHQ/Gimble-dev"
  version "1.0.0"
  url "https://github.com/gimbleHQ/Gimble-dev/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "b548437a563e36e948df2d9c06714203687854c2fc205b16d4f54b552d9d2674"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=1.0.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
