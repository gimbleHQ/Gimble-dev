class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.1"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.1.tar.gz"
  sha256 "d3a541485e60763ee5f822e7db036e53912d2863f3b0154825735608480aa7e0"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.1", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
