class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.4.0"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.4.0.tar.gz"
  sha256 "e64146e7961c9eee52f3baf76680b6c05af4462f815a0cee0c2a364b9f432996"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.4.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
