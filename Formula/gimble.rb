class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/gimbleHQ/Gimble-dev"
  version "1.0.0"
  url "https://github.com/gimbleHQ/Gimble-dev/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "8c15bcdccf7719ae3dc59b6fe62b89b3cc1cfb141611fd450503c60fc1f4b6f0"
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
