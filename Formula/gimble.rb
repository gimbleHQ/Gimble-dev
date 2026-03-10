class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.3"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.3.tar.gz"
  sha256 "ad8a863aa19851c22df10beebd6a798e5a480f8e3630fe5b2e405f1ba7c38a2b"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.3", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
