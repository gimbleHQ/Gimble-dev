class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.6"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.6.tar.gz"
  sha256 "d22d6a03ef3abb6c9b385d784567734ccccac7c3fefa74ae483f82e0730c0f96"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.6", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
