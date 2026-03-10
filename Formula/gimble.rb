class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.3"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.3.tar.gz"
  sha256 "64a803cefd25e0ebb83acc24874aed5873068496df2949b19d28d590867b98d7"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.3", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
