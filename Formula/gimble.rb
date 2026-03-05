class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.8"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.8.tar.gz"
  sha256 "c09adf17651a15ad7ccb8c44f22b6ed2f00aa3bd7b19a66345f260df897369bb"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.8", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
