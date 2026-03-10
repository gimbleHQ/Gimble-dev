class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.1"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.1.tar.gz"
  sha256 "beacfb6257ceda4efeb1429e0f19a74d05f95fc4371a5822501d5fd5716a8c2d"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.1", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
