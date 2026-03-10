class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.2"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.2.tar.gz"
  sha256 "867971617fbe0fa9fe11fc298d5431130c79e9914cbe7890cde5183189708ac8"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.2", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
