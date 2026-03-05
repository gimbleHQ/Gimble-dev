class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.11"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.11.tar.gz"
  sha256 "1a8c887514682a6aff0848afb112135c8543fb00a0e70885bd1eee6a7809be6b"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.11", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
