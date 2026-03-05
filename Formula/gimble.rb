class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.13"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.13.tar.gz"
  sha256 "f879eead6a1e028a8d98808872fd77270dfe70e6e3282e8e6c6567280f2be3fe"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.13", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
