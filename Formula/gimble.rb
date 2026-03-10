class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.4"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.4.tar.gz"
  sha256 "656eee078617d1110897f378bfa9f43dc4e23021ebd10bf2d71ed6b73bd1f948"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.4", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
