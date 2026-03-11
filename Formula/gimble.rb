class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.5"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.5.tar.gz"
  sha256 "49d52965dd1f63282d955f8fe0798ebc934c7f1f2a385d55d602e5f2c4f2a533"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.5", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
