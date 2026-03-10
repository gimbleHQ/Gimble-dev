class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.8"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.8.tar.gz"
  sha256 "97207a1f3944d09815d5143020e0e1089618de211616bbd25aabd5f13e113f6f"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.8", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
