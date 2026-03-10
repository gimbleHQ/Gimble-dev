class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.9"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.9.tar.gz"
  sha256 "59d4ed19ee9bc1dbe4e9b7ff8f3a1b665f386fb3d02c8a9ddd34e036f0fae675"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.9", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
