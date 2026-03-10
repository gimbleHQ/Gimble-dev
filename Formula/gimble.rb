class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.7"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.7.tar.gz"
  sha256 "5414159a504424387bcc755e52aa5ff067e90ddd74f8cb5c5bcf459738fe2d0b"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.7", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
