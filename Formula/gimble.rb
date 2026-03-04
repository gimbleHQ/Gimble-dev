class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.7"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.7.tar.gz"
  sha256 "9268a33a5a797a67062c4572f3577cb8555d297ee37ce3c826f4586c8368e898"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.7", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
