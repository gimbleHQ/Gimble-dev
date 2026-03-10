class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.4"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.4.tar.gz"
  sha256 "72cb4650930005a94873a3151d4233c79c0c48b1e86ecc78f50b825bfad82f09"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.4", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
