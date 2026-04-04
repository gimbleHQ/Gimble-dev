class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.6.0"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.6.0.tar.gz"
  sha256 "5948c961d6e9129a7d6f9295c21f8ab029230d3e6879c0199263a97eb96d522f"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.6.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
